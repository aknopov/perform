package net

import (
	"context"
	"errors"
	sysnet "net"
	"testing"
	"time"

	"github.com/aknopov/perform/mocker"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSimpleTCPPacket = []byte{
	0xb0, 0x3c, 0xdc, 0x87, 0xc8, 0x5a, 0xb0, 0x19, 0x21, 0xaf, 0x44, 0x10, 0x08, 0x00, 0x45, 0x00,
	0x00, 0x41, 0x29, 0xda, 0x40, 0x00, 0x36, 0x06, 0x66, 0xf4, 0x68, 0x12, 0x8a, 0x43, 0xc0, 0xa8,
	0x00, 0xeb, 0x01, 0xbb, 0x51, 0x2d, 0x0a, 0x34, 0x8e, 0x3e, 0x2a, 0xb3, 0xe6, 0x97, 0x50, 0x18,
	0x00, 0x0b, 0x23, 0x9d, 0x00, 0x00, 0x17, 0x03, 0x03, 0x00, 0x14, 0x3f, 0xee, 0x7e, 0xe6, 0x5c,
	0x1f, 0xdb, 0x81, 0x3a, 0x07, 0x75, 0xae, 0xcb, 0x76, 0x66, 0xb6, 0xe3, 0xa4, 0xbd, 0xaf,
}

var testSimpleTCPPacketRes = []byte{
	0xb0, 0x19, 0x21, 0xaf, 0x44, 0x10, 0xb0, 0x3c, 0xdc, 0x87, 0xc8, 0x5a, 0x08, 0x00, 0x45, 0x00,
	0x00, 0x28, 0x85, 0xd5, 0x40, 0x00, 0x80, 0x06, 0x89, 0xda, 0xc0, 0xa8, 0x00, 0xeb, 0x68, 0x12,
	0x8a, 0x43, 0x51, 0x2d, 0x01, 0xbb, 0xe1, 0x4b, 0xd2, 0x9d, 0x32, 0x49, 0x11, 0x33, 0x50, 0x14,
	0x00, 0x00, 0x6d, 0xb1, 0x00, 0x00}

var testICMP6 = []byte{
	0x24, 0xbe, 0x05, 0x27, 0x0b, 0x17, 0x00, 0x1f, 0xca, 0xb3, 0x75, 0xc0, 0x86, 0xdd, 0x6e, 0x00,
	0x00, 0x00, 0x00, 0x20, 0x3a, 0xff, 0xfe, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x1f,
	0xca, 0xff, 0xfe, 0xb3, 0x75, 0xc0, 0x26, 0x20, 0x00, 0x00, 0x10, 0x05, 0x00, 0x00, 0x26, 0xbe,
	0x05, 0xff, 0xfe, 0x27, 0x0b, 0x17, 0x87, 0x00, 0x1e, 0xba, 0x00, 0x00, 0x00, 0x00, 0x26, 0x20,
	0x00, 0x00, 0x10, 0x05, 0x00, 0x00, 0x26, 0xbe, 0x05, 0xff, 0xfe, 0x27, 0x0b, 0x17, 0x01, 0x01,
	0x00, 0x1f, 0xca, 0xb3, 0x75, 0xc0,
}

var testBogus = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0xff, 0xde, 0xad, 0xbe, 0xef,
}

var testSmallUDPPacket = []byte{
	0x01, 0x00, 0x5e, 0x00, 0x00, 0xfb, 0x00, 0x1b, 0xa9, 0x53, 0xe0, 0x91, 0x08, 0x00, 0x45, 0x00,
	0x00, 0x4d, 0x3c, 0xb1, 0x00, 0x00, 0xff, 0x11, 0xdc, 0xc6, 0xc0, 0xa8, 0x00, 0x84, 0xe0, 0x00,
	0x00, 0xfb, 0x14, 0xe5, 0x14, 0xe9, 0x00, 0x39, 0xfe, 0x31, 0x00, 0x00, 0x84, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x0f, 0x42, 0x52, 0x4e, 0x30, 0x30, 0x31, 0x42, 0x41, 0x39,
	0x35, 0x33, 0x45, 0x30, 0x39, 0x31, 0x05, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x00, 0x00, 0x01, 0x80,
	0x01, 0x00, 0x00, 0x00, 0xf0, 0x00, 0x04, 0xc0, 0xa8, 0x00, 0x84,
}

var testDecodeOptions = gopacket.DecodeOptions{
	SkipDecodeRecovery: true,
	NoCopy:             true,
}

var (
	tcpPacket    = gopacket.NewPacket(testSimpleTCPPacket, layers.LinkTypeEthernet, testDecodeOptions)
	tcpPacketRes = gopacket.NewPacket(testSimpleTCPPacketRes, layers.LinkTypeEthernet, testDecodeOptions)
	udpPacket    = gopacket.NewPacket(testSmallUDPPacket, layers.LinkTypeEthernet, testDecodeOptions)
	icmp6Packet  = gopacket.NewPacket(testICMP6, layers.LinkTypeEthernet, testDecodeOptions)
	bogusPacket  = gopacket.NewPacket(testBogus, layers.LinkTypeEthernet, testDecodeOptions)
)

func TestKeepTracing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	assert.True(t, keepTracing(ctx))

	cancel()
	assert.False(t, keepTracing(ctx))
}

func TestWaitNextPacket(t *testing.T) {
	pChan := make(pCapChannel)
	go func() { pChan <- tcpPacket }()
	time.Sleep(5 * time.Millisecond)

	start := time.Now()
	aPacket, aChan := waitNextPacket([]pCapChannel{pChan})
	assert.Equal(t, tcpPacket, aPacket)
	assert.Equal(t, pChan, aChan)
	assert.Less(t, time.Since(start), 5*time.Millisecond)

	start = time.Now()
	aPacket, aChan = waitNextPacket([]pCapChannel{pChan})
	assert.Nil(t, aPacket)
	assert.Nil(t, aChan)
	assert.GreaterOrEqual(t, time.Since(start), CHECK_DONE_INTVL)
}

func TestDecodeIp(t *testing.T) {
	var srcIP, dstIP string

	assert.True(t, decodeIP(tcpPacket, &srcIP, &dstIP))
	assert.Equal(t, "104.18.138.67", srcIP)
	assert.Equal(t, "192.168.0.235", dstIP)

	assert.True(t, decodeIP(icmp6Packet, &srcIP, &dstIP))
	assert.Equal(t, "fe80::21f:caff:feb3:75c0", srcIP)
	assert.Equal(t, "2620:0:1005:0:26be:5ff:fe27:b17", dstIP)

	assert.False(t, decodeIP(bogusPacket, &srcIP, &dstIP))
}

func TestDecodeTcp(t *testing.T) {
	var srcAddr, dstAddr net.Addr

	assert.True(t, decodeTCP(tcpPacket, &srcAddr, &dstAddr))
	assert.Equal(t, net.Addr{IP: "104.18.138.67", Port: 443}, srcAddr)
	assert.Equal(t, net.Addr{IP: "192.168.0.235", Port: 20781}, dstAddr)

	assert.False(t, decodeTCP(bogusPacket, &srcAddr, &dstAddr))
	assert.False(t, decodeTCP(udpPacket, &srcAddr, &dstAddr))
}

func TestDecodeUdp(t *testing.T) {
	var srcAddr, dstAddr net.Addr

	assert.True(t, decodeUDP(udpPacket, &srcAddr, &dstAddr))
	assert.Equal(t, net.Addr{IP: "192.168.0.132", Port: 5349}, srcAddr)
	assert.Equal(t, net.Addr{IP: "224.0.0.251", Port: 5353}, dstAddr)

	assert.False(t, decodeUDP(bogusPacket, &srcAddr, &dstAddr))
	assert.False(t, decodeUDP(tcpPacket, &srcAddr, &dstAddr))
}

func TestFindActiveDevices(t *testing.T) {
	mockDevs := []pcap.Interface{
		{Name: "any", Description: "Pseudo-device", Flags: 0x36, Addresses: []pcap.InterfaceAddress{}},
		{Name: "bluetooth0t", Description: "Bluetooth Device", Flags: 0x2e, Addresses: []pcap.InterfaceAddress{{}}},
		{Name: "wlo1", Description: "Wi-Fi", Flags: 0x1e, Addresses: []pcap.InterfaceAddress{{}}},
	}
	mockFindDev := func() ([]pcap.Interface, error) {
		return mockDevs, nil
	}

	devs := findActiveDevices(mockFindDev)
	assert.Len(t, devs, 1)
	assert.Equal(t, "wlo1", devs[0].Name)
}

func TestFindActiveDevicesFail(t *testing.T) {
	defer mocker.ReplaceItem(&errChan, make(chan error))()

	mockFindDev := func() ([]pcap.Interface, error) {
		return []pcap.Interface{}, errors.New("test")
	}
	dataChan := make(chan []pcap.Interface)

	go func() { dataChan <- findActiveDevices(mockFindDev) }()

	require.Error(t, <-errChan)
	assert.Empty(t, <-dataChan)
}

func TestSortAddresses(t *testing.T) {
	addr1 := net.Addr{IP: "192.168.0.235", Port: 49671}
	addr2 := net.Addr{IP: "31.13.80.53", Port: 443}
	addr3 := net.Addr{IP: "127.0.0.1", Port: 49667}
	addr4 := net.Addr{IP: "127.0.0.1", Port: 1042}
	addr5 := net.Addr{IP: "172.28.216.133", Port: 5432}
	addr6 := net.Addr{IP: "192.168.0.180", Port: 137}
	addr7 := net.Addr{IP: "192.168.0.255", Port: 137}
	mask1 := sysnet.IPMask{255, 255, 255, 0}
	mask2 := sysnet.IPMask{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0}

	dev1 := pcap.Interface{Name: "eth0", Addresses: []pcap.InterfaceAddress{
		{IP: sysnet.ParseIP("192.168.0.235"), Netmask: mask1},
		{IP: sysnet.ParseIP("fe80::7e0d:16a6:d0c9:2b9a"), Netmask: mask2},
	}}
	dev2 := pcap.Interface{Name: "lo", Addresses: []pcap.InterfaceAddress{{IP: sysnet.ParseIP("127.0.0.1")}, {IP: sysnet.ParseIP("::1")}}}

	lAddr, rAddr := sortAddresses(&addr1, &addr2, &dev1)
	assert.Equal(t, toSlice(&addr1, &addr2), toSlice(lAddr, rAddr))
	lAddr, rAddr = sortAddresses(&addr2, &addr1, &dev1)
	assert.Equal(t, toSlice(&addr1, &addr2), toSlice(lAddr, rAddr))

	lAddr, rAddr = sortAddresses(&addr3, &addr4, &dev2)
	assert.Equal(t, toSlice(&addr3, &addr4), toSlice(lAddr, rAddr))
	lAddr, rAddr = sortAddresses(&addr4, &addr3, &dev2)
	assert.Equal(t, toSlice(&addr3, &addr4), toSlice(lAddr, rAddr))

	// ambiguous case - local connection
	lAddr, rAddr = sortAddresses(&addr6, &addr7, &dev1)
	assert.Equal(t, toSlice(&addr6, &addr7), toSlice(lAddr, rAddr))
	lAddr, rAddr = sortAddresses(&addr7, &addr6, &dev1)
	assert.Equal(t, toSlice(&addr7, &addr6), toSlice(lAddr, rAddr))

	lAddr, rAddr = sortAddresses(&addr3, &addr5, &dev1)
	assert.Nil(t, lAddr)
	assert.Nil(t, rAddr)
}

func TestProcessPacket(t *testing.T) {
	defer mocker.ReplaceItem(&procConnMap, make(map[net.Addr]*procNetStat))()

	mockDev := pcap.Interface{Name: "tst", Addresses: []pcap.InterfaceAddress{{IP: sysnet.IP{192, 168, 0, 235}, Netmask: sysnet.IPMask{255, 255, 255, 255}}}}
	expAddr := net.Addr{IP: "192.168.0.235", Port: 20781}

	processPacket(tcpPacket, &mockDev)
	assert.Len(t, procConnMap, 1)
	assert.Contains(t, procConnMap, expAddr)
	ps := procConnMap[expAddr]
	assert.EqualValues(t, -1, ps.pid)
	assert.EqualValues(t, 1, ps.netCounters.PacketsRecv)
	assert.EqualValues(t, 79, ps.netCounters.BytesRecv)
	assert.Zero(t, ps.netCounters.Errin)
	assert.Zero(t, ps.netCounters.PacketsSent)
	assert.Zero(t, ps.netCounters.BytesSent)
	assert.Zero(t, ps.netCounters.Errout)
	assert.NotZero(t, ps.lastUpdate)

	processPacket(tcpPacketRes, &mockDev)
	assert.Len(t, procConnMap, 1)
	assert.Contains(t, procConnMap, expAddr)
	assert.EqualValues(t, 1, ps.netCounters.PacketsRecv)
	assert.EqualValues(t, 79, ps.netCounters.BytesRecv)
	assert.EqualValues(t, 1, ps.netCounters.PacketsSent)
	assert.EqualValues(t, 54, ps.netCounters.BytesSent)
}

func BenchmarkFindActiveDevices(b *testing.B) {
	for range b.N {
		findActiveDevices(pcap.FindAllDevs)
	}
}

func toSlice[T any](vals ...T) []T {
	return vals
}
