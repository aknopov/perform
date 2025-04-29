package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aknopov/perform"
	pm "github.com/aknopov/perform/cmd/param"
	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

var (
	NO_TIMESTAT = &cpu.TimesStat{}
	NO_MEMSTAT  = &process.MemoryInfoStat{}
	NO_NET_IO   = []net.IOCountersStat{}
)

var (
	getProcessList = ps.Processes
	findProcess    = ps.FindProcess
	getNumCPU      = runtime.NumCPU
)

func main() {
	procId, paramList, refreshSec, err := pm.ParseParams(os.Args, func() { usage(os.Stderr) })
	if err != nil {
		if err.Error() != "flag: help requested" {
			fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
			usage(os.Stderr)
		}
		os.Exit(1)
	}
	refreshPeriod := time.Duration(int64(refreshSec * float64(time.Second)))

	// hostInfo := perform.AssertNoErr(host.Info())

	pid := getProcPid(procId)
	p, _ := process.NewProcess(int32(pid))

	pm.PrintHeader(os.Stdout, paramList)
	pollStats(pm.NewQProcess(p), paramList, refreshPeriod)
}

func pollStats(proc pm.IQProcess, paramList pm.ParamList, refreshPeriod time.Duration) {

	getProcNetFn := func() ([]net.IOCountersStat, error) { return proc.NetIOCounters(false) }
	queryNet := checkIfNetAsked(paramList)
	netInfo := NO_NET_IO

	ticker := time.NewTicker(refreshPeriod)
	values := make([]float64, len(paramList))

	for range ticker.C {
		if !isProcessAlive(int(proc.GetPID())) {
			break
		}

		if queryNet {
			netInfo = perform.AssumeOnErr(getProcNetFn, NO_NET_IO)
		}

		for i, p := range paramList {
			values[i] = getValue(proc, netInfo, p)
		}

		pm.PrintValues(os.Stdout, paramList, values)
	}
}

func checkIfNetAsked(paramList pm.ParamList) bool {
	for _, p := range paramList {
		if p == pm.Tx || p == pm.Rx {
			return true
		}
	}
	return false
}

var (
	prevRx uint64 = 0
	prevTx uint64 = 0
)

func getValue(proc pm.IQProcess, netInfo []net.IOCountersStat, p pm.ParamType) float64 {

	switch p {
	case pm.Cpu:
		ts := perform.AssumeOnErr(proc.Times, NO_TIMESTAT)
		return ts.User + ts.System // Also: Total()
	case pm.Mem:
		memInfo := perform.AssumeOnErr(proc.MemoryInfo, NO_MEMSTAT)
		return float64(memInfo.RSS) / 1024
	case pm.CPUs:
		return float64(getNumCPU())
	case pm.PIDs:
		return float64(perform.AssumeOnErr(proc.NumThreads, -1))
	case pm.Tx:
		var txBytes uint64
		for _, ni := range netInfo {
			txBytes += ni.BytesSent
		}
		if prevTx == 0 {
			prevTx = txBytes
		}
		return float64(txBytes-prevTx) / 1024
	case pm.Rx:
		var rxBytes uint64
		for _, ni := range netInfo {
			rxBytes += ni.BytesRecv
		}
		if prevRx == 0 {
			prevRx = rxBytes
		}
		return float64(rxBytes-prevRx) / 1024
	default:
		panic(fmt.Errorf("unknown parameter type: %v", p))
	}
}

func getProcPid(cmd string) int {
	procList := perform.AssertNoErr(getProcessList())

	pid, err := strconv.Atoi(cmd)

	for _, p := range procList {
		if err == nil && pid == p.Pid() {
			return pid
		} else if strings.Contains(p.Executable(), cmd) {
			return p.Pid()
		}
	}

	fmt.Fprintf(os.Stderr, "Can't find process with PID or command line '%s'\n", cmd)
	os.Exit(1)
	
	return -1
}

func isProcessAlive(pid int) bool {
	proc, err := findProcess(pid)
	return err == nil && proc != nil && proc.Pid() == pid
}

//nolint:errcheck
func usage(sink *os.File) {
	fmt.Fprintln(sink, `Application performance statistics
Usage: proc-stat -refresh=... -params=... proc
proc - process ID or command line
-refresh - interval in seconds (default 1.0 sec)
-params - comma separated list of:
  Cpu - total CPU time (msec) spent on runing process
  Mem - process memory usage (KB)
  PIDs - number of process threads
  CPUs - number of host processors available to the process
  Rx - total network read rate (KB)
  Tx - total network write rate (KB)`)
}
