package net

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
)

// Reduced net.IOCountersStat
type IOCountersStat struct {
	Name        string `json:"name"`        // interface name
	BytesSent   uint64 `json:"bytesSent"`   // number of bytes sent
	BytesRecv   uint64 `json:"bytesRecv"`   // number of bytes received
	PacketsSent uint64 `json:"packetsSent"` // number of packets sent
	PacketsRecv uint64 `json:"packetsRecv"` // number of packets received
	Errin       uint64 `json:"errin"`       // total number of errors while receiving
	Errout      uint64 `json:"errout"`      // total number of errors while sending
}

// Network IO counters for a process
type procNetStat struct {
	Pid         int32          `json:"pid"`        // process PID
	NetCounters IOCountersStat `json:"netCounts"`  // process network counters
	RemoteAddr  net.Addr       `json:"remoteAddr"` // remote address
	LastUpdate  time.Time      `json:"lastUpd"`    // last updated
}

// For unit test mocking
type connProviderF func(context.Context, string) ([]net.ConnectionStat, error)

var (
	procConnMap      map[net.Addr]*procNetStat
	watchLock        sync.RWMutex
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
	tracePackets(ctx)

	return errChan
}

// GetProcessNetIOCounters returns NetIOCounters of the process.
func GetProcessNetIOCounters() ([]IOCountersStat, error) {
	lclConMap := getProcConnMapCopy()
	countsMap := make(map[string]IOCountersStat)
	fmt.Fprintf(os.Stderr, "Requesting process connections\n") // UC debug
	for _, ps := range lclConMap {
		if ps.Pid != pid {
			continue
		}

		iName := ps.NetCounters.Name
		if stat, ok := countsMap[iName]; ok {
			sumStats(&stat, &ps.NetCounters)
		} else {
			countsMap[iName] = ps.NetCounters
		}
	}

	if len(countsMap) == 0 {
		return nil, fmt.Errorf("no net counters for pid %d", pid)
	}

	ret := make([]IOCountersStat, 0)
	for _, cs := range countsMap {
		if len(ret) == 0 {
			ret = append(ret, cs)
		} else {
			sumStats(&ret[0], &cs)
		}
	}

	return ret, nil
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
	watchLock.RLock()
	defer watchLock.RUnlock()

	newMap := make(map[net.Addr]procNetStat)
	for key, value := range procConnMap {
		newMap[key] = *value
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

func updateTable(ctx context.Context, pid int32, connProvider connProviderF, expiry time.Duration) { // UC add pid to args
	conns, err := connProvider(ctx, "all")
	if err != nil {
		errChan <- err
		return
	}
	// fmt.Fprintf(os.Stderr, "Discovered %d connections\n", len(conns)) // UC Debug

	tempAddrMap := make(map[net.Addr]net.ConnectionStat)
	// Leave only active connections of the process
	for _, conn := range conns {
		if _, ok := inactiveStatuses[conn.Status]; !ok && (pid == -1 || pid == conn.Pid) {
			tempAddrMap[conn.Laddr] = conn
		}
	}

	watchLock.Lock()

	ntrans := 0
	// remove entries for closed connections
	for a, ps := range procConnMap {
		if _, ok := tempAddrMap[a]; !ok && time.Since(ps.LastUpdate) > expiry {
			// fmt.Fprintf(os.Stderr, "Removing conversation of process %d to %v\n", ps.Pid, ps.RemoteAddr) // UC debug
			delete(procConnMap, a)
		} else if ok && ps.Pid == -1 {
			ntrans++
		}
	}

	// add new connections
	for a, c := range tempAddrMap {
		ps, ok := procConnMap[a]
		if !ok { // UC fatal error: concurrent map read and map write
			// fmt.Fprintf(os.Stderr, "Adding conversation of process %d to %v\n", c.Pid, c.Raddr) // UC debug
			procConnMap[a] = &procNetStat{Pid: c.Pid, NetCounters: IOCountersStat{}, RemoteAddr: c.Raddr, LastUpdate: time.Now()}
		} else if ps.Pid == -1 {
			ps.Pid = c.Pid
			ntrans--
		}
	}

	 watchLock.Unlock()

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
	for _, ps := range procConnMap {
		if ps.Pid == -1 {
			if ar, ok := tempAddrMap[ps.RemoteAddr]; ok {
				fmt.Fprintf(os.Stderr, "Guessed connection for process %d to %v\n", ps.Pid, ps.RemoteAddr) // UC  debug
				ps.Pid = ar.Pid
			}
		}
	}
}
