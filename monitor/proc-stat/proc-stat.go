package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aknopov/perform"
	"github.com/aknopov/perform/monitor"
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

func main() {
	cmd, paramList, refreshSec, err := monitor.ParseParams(os.Args, func() { usage(os.Stderr) })
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
		usage(os.Stderr)
		os.Exit(1)
	}
	refreshPeriod := time.Duration(int64(refreshSec * float64(time.Second)))

	// hostInfo := perform.AssertNoErr(host.Info())

	pid := perform.AssertNoErr(getProcPid(cmd))
	p, _ := process.NewProcess(int32(pid))

	monitor.PrintHeader(os.Stdout, paramList)
	pollStats(p, paramList, refreshPeriod)
}

func pollStats(proc *process.Process, paramList *monitor.ParamList, refreshPeriod time.Duration) {

	// _ = perform.AssertNoErr(p.Percent(0))

	getProcNetFn := func() ([]net.IOCountersStat, error) { return proc.NetIOCounters(false) }
	queryNet := checkIfNetAsked(paramList)
	netInfo := NO_NET_IO

	ticker := time.NewTicker(refreshPeriod)
	values := make([]float64, len(*paramList))

	for range ticker.C {
		if !isProcessAlive(int(proc.Pid)) {
			break
		}

		if queryNet {
			netInfo = perform.AssumeOnErr(getProcNetFn, NO_NET_IO)
		}

		for i, p := range *paramList {
			values[i] = getValue(proc, netInfo, p)
		}

		monitor.PrintValues(os.Stdout, values)
	}
}

func checkIfNetAsked(paramList *monitor.ParamList) bool {
	for _, p := range *paramList {
		if p == monitor.Tx || p == monitor.Rx {
			return true
		}
	}
	return false
}

func getValue(proc *process.Process, netInfo []net.IOCountersStat, p monitor.ParamType) float64 {

	switch p {
	case monitor.Cpu:
		ts := perform.AssumeOnErr(proc.Times, NO_TIMESTAT)
		return ts.User + ts.System // Also: Total()
	case monitor.Mem:
		memInfo := perform.AssumeOnErr(proc.MemoryInfo, NO_MEMSTAT)
		return float64(memInfo.RSS) / 1024
	case monitor.CPUs:
		return float64(runtime.NumCPU())
	case monitor.PIDs:
		return float64(perform.AssumeOnErr(proc.NumThreads, -1))
	case monitor.Tx:
		var txBytes uint64
		for _, ni := range netInfo {
			txBytes += ni.BytesSent
		}
		return float64(txBytes)
	case monitor.Rx:
		var rxBytes uint64
		for _, ni := range netInfo {
			rxBytes += ni.BytesRecv
		}
		return float64(rxBytes)
	default:
		panic(fmt.Errorf("Unknown parameter type: %v", p))
	}
}

func getProcPid(cmd string) (int, error) {
	procList := perform.AssertNoErr(ps.Processes())

	pid, err := strconv.Atoi(cmd)

	for _, p := range procList {
		if err == nil && pid == p.Pid() {
			return pid, nil
		} else if strings.Contains(p.Executable(), cmd) && isProcessAlive(p.Pid()) {
			return p.Pid(), nil
		}
	}

	return -1, fmt.Errorf("can't find process with '%s'", cmd)
}

func isProcessAlive(pid int) bool {
	proc, err := ps.FindProcess(pid)
	return err == nil && proc.Pid() == pid
}

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
  Rx - total network read rate (KBs)
  Tx - total network write rate (KBs)`)
}
