package net

import (
	"context"
	"errors"
	sysnet "net"
	"slices"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/shirou/gopsutil/v4/net"
)

const (
	PCAP_IF_LOOPBACK                         = uint32(0x00000001)
	PCAP_IF_UP                               = uint32(0x00000002)
	PCAP_IF_RUNNING                          = uint32(0x00000004)
	PCAP_IF_WIRELESS                         = uint32(0x00000008)
	PCAP_IF_CONNECTION_STATUS_UNKNOWN        = uint32(0x00000000)
	PCAP_IF_CONNECTION_STATUS_CONNECTED      = uint32(0x00000010)
	PCAP_IF_CONNECTION_STATUS_DISCONNECTED   = uint32(0x00000020)
	PCAP_IF_CONNECTION_STATUS_NOT_APPLICABLE = uint32(0x00000030)
	PCAP_GTG_FLAGS                           = PCAP_IF_UP | PCAP_IF_RUNNING | PCAP_IF_CONNECTION_STATUS_CONNECTED
)

const (
	CHECK_DONE_INTVL = 10 * time.Millisecond
)

// For unit test mocking
type (
	findDevsF   func() ([]pcap.Interface, error)
	openLiveF   func(_ string, _ int32, _ bool, _ time.Duration) (*pcap.Handle, error)
	pCapChannel chan gopacket.Packet
)

var (
	findAllDevs = pcap.FindAllDevs
)

func tracePackets(ctx context.Context) {
	devs := findActiveDevices(findAllDevs)
	if len(devs) == 0 {
		errChan <- errors.New("no active network devices found")
		return
	}

	processDeviceMsgs(ctx, devs, pcap.OpenLive)
}

func findActiveDevices(fFindDevs findDevsF) []pcap.Interface {
	ret := make([]pcap.Interface, 0)

	devs, err := fFindDevs()
	if err != nil {
		errChan <- err
		return ret
	}

	for _, dev := range devs {
		if (dev.Flags&PCAP_GTG_FLAGS) == PCAP_GTG_FLAGS && len(dev.Addresses) > 0 {
			ret = append(ret, dev)
		}
	}

	return ret
}

func processDeviceMsgs(ctx context.Context, devs []pcap.Interface, fOpenLive openLiveF) {
	var handle *pcap.Handle
	var err error

	packetChnls := make([]pCapChannel, 0)
	channelDevs := make(map[pCapChannel]*pcap.Interface)
	for _, dev := range devs {
		if handle, err = fOpenLive(dev.Name, 1600, false, 1*time.Second); err != nil {
			errChan <- err
			continue
		}
		defer handle.Close()

		if err = handle.SetBPFFilter("tcp || udp"); err != nil {
			errChan <- err
			continue
		}

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		packetCh := packetSource.Packets()

		packetChnls = append(packetChnls, packetCh)
		channelDevs[packetCh] = &dev
	}

	for keepTracing(ctx) {
		p, ch := waitNextPacket(packetChnls)
		if p == nil {
			continue
		}

		dev := channelDevs[ch]
		processPacket(p, dev)
	}
}

func ipMatch(ip sysnet.IP, nicAddr pcap.InterfaceAddress) bool {
	return slices.Equal(ip.Mask(nicAddr.Netmask), nicAddr.IP.Mask(nicAddr.Netmask))
}

// Figures which address is local and which remote
func sortAddresses(addr1 *net.Addr, addr2 *net.Addr, dev *pcap.Interface) (*net.Addr, *net.Addr) {
	ip1 := sysnet.ParseIP(addr1.IP)
	ip2 := sysnet.ParseIP(addr2.IP)
	for _, nicAddr := range dev.Addresses {
		switch {
		case ipMatch(ip1, nicAddr) && !ipMatch(ip2, nicAddr):
			return addr1, addr2
		case ipMatch(ip2, nicAddr) && !ipMatch(ip1, nicAddr):
			return addr2, addr1
		case ipMatch(ip1, nicAddr) && addr1.Port >= addr2.Port:
			return addr1, addr2
		case ipMatch(ip2, nicAddr) && addr1.Port < addr2.Port:
			return addr2, addr1
		}
	}
	return nil, nil
}

func addTransientConn(srcAddr *net.Addr, dstAddr *net.Addr, dev *pcap.Interface) *net.Addr {
	lclAddr, rmtAddr := sortAddresses(srcAddr, dstAddr, dev)
	if lclAddr == nil || rmtAddr == nil {
		return nil
	}

	procConnMap[*lclAddr] = createNetStat(-1, IOCountersStat{})
	return lclAddr
}

func processPacket(p gopacket.Packet, dev *pcap.Interface) {
	var errCnt uint64
	errLayer := p.ErrorLayer()
	if errLayer != nil {
		// What about errLayer.Error()?
		errCnt = 1
	}

	nBytes := uint64(len(p.Data()))
	var dstAddr, srcAddr net.Addr
	if decodeTCP(p, &srcAddr, &dstAddr) || decodeUDP(p, &srcAddr, &dstAddr) {

		connMapLock.Lock()
		statOut, isOut := procConnMap[srcAddr]
		statIn, isIn := procConnMap[dstAddr]
		if !isIn && !isOut {
			addTransientConn(&srcAddr, &dstAddr, dev)
			statOut, isOut = procConnMap[srcAddr]
			statIn, isIn = procConnMap[dstAddr]
		}
		connMapLock.Unlock()

		if isOut {
			statOut.lock.Lock()
			statOut.netCounters.BytesSent += nBytes
			statOut.netCounters.PacketsSent++
			statOut.netCounters.Errout += errCnt
			statOut.lastUpdate = time.Now()
			statOut.lock.Unlock()
		} else if isIn {
			statIn.lock.Lock()
			statIn.netCounters.BytesRecv += nBytes
			statIn.netCounters.PacketsRecv++
			statIn.netCounters.Errin += errCnt
			statIn.lastUpdate = time.Now()
			statIn.lock.Unlock()
		}
	}
}

func keepTracing(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}

func waitNextPacket(packetChnls []pCapChannel) (gopacket.Packet, pCapChannel) {
	for _, packetCh := range packetChnls {
		select {
		case p := <-packetCh:
			return p, packetCh
		default:
			continue
		}
	}

	time.Sleep(CHECK_DONE_INTVL)
	return nil, nil
}

func decodeTCP(p gopacket.Packet, srcAddr *net.Addr, dstAddr *net.Addr) bool {
	var srcIP, dstIP string
	if !decodeIP(p, &srcIP, &dstIP) {
		return false
	}

	tcpLayer := p.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return false
	}

	tcp := tcpLayer.(*layers.TCP)
	*srcAddr = net.Addr{IP: srcIP, Port: uint32(tcp.SrcPort)}
	*dstAddr = net.Addr{IP: dstIP, Port: uint32(tcp.DstPort)}
	return true
}

func decodeUDP(p gopacket.Packet, srcAddr *net.Addr, dstAddr *net.Addr) bool {
	var srcIP, dstIP string
	if !decodeIP(p, &srcIP, &dstIP) {
		return false
	}

	udpLayer := p.Layer(layers.LayerTypeUDP)
	if udpLayer == nil {
		return false
	}

	udp := udpLayer.(*layers.UDP)
	*srcAddr = net.Addr{IP: srcIP, Port: uint32(udp.SrcPort)}
	*dstAddr = net.Addr{IP: dstIP, Port: uint32(udp.DstPort)}
	return true
}

func decodeIP(p gopacket.Packet, srcIP *string, dstIP *string) bool {
	ip4Layer := p.Layer(layers.LayerTypeIPv4)
	if ip4Layer != nil {
		ip := ip4Layer.(*layers.IPv4)
		*srcIP = ip.SrcIP.String()
		*dstIP = ip.DstIP.String()
		return true
	}

	ip6Layer := p.Layer(layers.LayerTypeIPv6)
	if ip6Layer != nil {
		ip := ip6Layer.(*layers.IPv6)
		*srcIP = ip.SrcIP.String()
		*dstIP = ip.DstIP.String()
		return true
	}

	return false
}
