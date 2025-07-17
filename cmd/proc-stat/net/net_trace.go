package net

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

// Network data counters (reduced psutils IOCountersStat)
type IOCountersStat struct {
	BytesSent   uint64 `json:"bytesSent"`   // number of bytes sent
	BytesRecv   uint64 `json:"bytesRecv"`   // number of bytes received
	PacketsSent uint64 `json:"packetsSent"` // number of packets sent
	PacketsRecv uint64 `json:"packetsRecv"` // number of packets received
	Errin       uint64 `json:"errin"`       // total number of errors while receiving
	Errout      uint64 `json:"errout"`      // total number of errors while sending
}

// Network IO counters for a process
type procNetStat struct {
	pid         int32          // process PID
	netCounters IOCountersStat // process network counters
	remoteAddr  net.Addr       // remote address
	lastUpdate  time.Time      // last updated
	lock        *sync.Mutex    // modifications guard
}

// For unit test mocking
type connProviderF func(context.Context, string) ([]net.ConnectionStat, error)

var (
	procConnMap      map[net.Addr]*procNetStat
	connMapLock      sync.RWMutex
	errChan          chan error
	pid              int32
	void             = struct{}{}
	inactiveStatuses = map[string]struct{}{"CLOSED": void, "CLOSE": void, "TIME_WAIT": void, "DELETE": void}
)

// Start collecting information on open ports and network traffic with Go-Pcap
func StartTracing(ctx context.Context, ppid int32, intvl time.Duration) chan error {
	if procConnMap != nil {
		panic("Repeated capturing of process NET I/O is not supported")
	}

	procConnMap = make(map[net.Addr]*procNetStat)
	errChan = make(chan error)

	pid = ppid
	go pollNetStat(ctx, ppid, intvl)
	go tracePackets(ctx)

	return errChan
}

// GetProcessNetIOCounters sums all registered conversations
func GetProcessNetIOCounters() (*IOCountersStat, error) {
	lclConMap := getProcConnMapCopy()
	var ret *IOCountersStat
	for _, ps := range lclConMap {
		if ps.pid != pid {
			continue
		}

		if ret == nil {
			ret = new(IOCountersStat)
		}

		sumStats(ret, &ps.netCounters)
	}

	var err error
	if ret == nil {
		err = fmt.Errorf("no net counters for pid %d", pid)
	}

	return ret, err
}

func createNetStat(pid int32, remoteAddr net.Addr, netCounters IOCountersStat) *procNetStat {
	return &procNetStat{pid: pid, remoteAddr: remoteAddr, netCounters: netCounters, lastUpdate: time.Now(), lock: new(sync.Mutex)}
}

func sumStats(stat1 *IOCountersStat, stat2 *IOCountersStat) {
	stat1.BytesSent += stat2.BytesSent
	stat1.BytesRecv += stat2.BytesRecv
	stat1.PacketsSent += stat2.PacketsSent
	stat1.PacketsRecv += stat2.PacketsRecv
	stat1.Errin += stat2.Errin
	stat1.Errout += stat2.Errout
}

// Deep copy of ProcConnMap!
func getProcConnMapCopy() map[net.Addr]procNetStat {
	connMapLock.RLock()
	defer connMapLock.RUnlock()

	newMap := make(map[net.Addr]procNetStat)
	for key, value := range procConnMap {
		value.lock.Lock()
		newMap[key] = *value
		value.lock.Unlock()
	}
	return newMap
}

func pollNetStat(ctx context.Context, pid int32, intvl time.Duration) {
	watchTicker := time.NewTicker(intvl)

	for range watchTicker.C {
		select {
		case <-ctx.Done():
			return
		default:
			updateTable(ctx, pid, net.ConnectionsWithContext, 2*intvl)
		}
	}
}

func updateTable(ctx context.Context, pid int32, connProvider connProviderF, expiry time.Duration) {
	conns, err := connProvider(ctx, "all")
	if err != nil {
		errChan <- err
		return
	}

	tempAddrMap := make(map[net.Addr]net.ConnectionStat)
	// Leave only active connections of the process
	for _, conn := range conns {
		if _, ok := inactiveStatuses[conn.Status]; !ok && (pid == -1 || pid == conn.Pid) {
			tempAddrMap[conn.Laddr] = conn
		}
	}

	connMapLock.Lock()

	ntrans := 0
	// remove entries for closed connections
	for a, ps := range procConnMap {
		ps.lock.Lock()
		if _, ok := tempAddrMap[a]; !ok && ps.pid == -1 && time.Since(ps.lastUpdate) > expiry {
			delete(procConnMap, a)
		} else if ok && ps.pid == -1 {
			ntrans++
		}
		ps.lock.Unlock()
	}

	// add new connections
	for a, c := range tempAddrMap {
		ps, ok := procConnMap[a]
		if !ok {
			procConnMap[a] = createNetStat(c.Pid, c.Raddr, IOCountersStat{})
		} else if ps.pid == -1 {
			ps.pid = c.Pid
			ntrans--
		}
	}

	connMapLock.Unlock()

	// Deal with remained transient connections
	if ntrans > 0 {
		guessPidByRemote(conns)
	}
}

// Here we are doing "best effort" to figure process for a connection when it is opened and closed in between two polls.
// Pid is guessed by assuming that the application connects to the same remote endpoint repeatedly.
// (This will be invalid if several applications connect to the same endpoint - hence the "guess")
func guessPidByRemote(conns []net.ConnectionStat) {
	tempAddrMap := make(map[net.Addr]net.ConnectionStat)
	for _, conn := range conns {
		tempAddrMap[conn.Raddr] = conn
	}

	connMapLock.RLock()
	defer connMapLock.RUnlock()

	for _, ps := range procConnMap {
		if ps.pid == -1 {
			if ar, ok := tempAddrMap[ps.remoteAddr]; ok {
				ps.pid = ar.Pid
			}
		}
	}
}
