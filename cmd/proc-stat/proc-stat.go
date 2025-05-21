package main

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aknopov/perform"
	"github.com/aknopov/perform/cmd/cpushare"
	pm "github.com/aknopov/perform/cmd/param"

	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

var (
	NO_TIMESTAT = &cpu.TimesStat{}
	NO_MEMSTAT  = &process.MemoryInfoStat{}
)

// Function substitutions for unit tests
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

func reduceArg[A any, R any](f func(A) (R, error), arg A) func() (R, error) {
	return func() (R, error) { return f(arg) }
}

func pollStats(proc pm.IQProcess, paramList pm.ParamList, refreshPeriod time.Duration) {

	getProcNetFn := reduceArg(proc.NetIOCounters, false)
	queryNet := slices.Contains(paramList, pm.Rx) || slices.Contains(paramList, pm.Tx)
	netInfo := pm.NO_NET_IO

	ticker := time.NewTicker(refreshPeriod)
	values := make([]float64, len(paramList))

	for range ticker.C {
		if !isProcessAlive(int(proc.GetPID())) {
			break
		}

		if queryNet {
			netInfo = perform.AssumeOnErr(getProcNetFn, pm.NO_NET_IO)
		}

		for i, p := range paramList {
			values[i] = getValue(proc, netInfo, p)
		}

		pm.PrintValues(os.Stdout, paramList, values)
	}
}

func getValue(proc pm.IQProcess, netInfo []net.IOCountersStat, p pm.ParamType) float64 {

	provideCpuPerc := func() float64 { return perform.AssumeOnErr(reduceArg(proc.Percent, 0), 0) / float64(getNumCPU()) }
	switch p {
	case pm.Cpu:
		ts := perform.AssumeOnErr(proc.Times, NO_TIMESTAT)
		return ts.User + ts.System // Also: Total()
	case pm.CpuPerc:
		return cpushare.GetCpuPerc(provideCpuPerc)
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
		return float64(txBytes) / 1024
	case pm.Rx:
		var rxBytes uint64
		for _, ni := range netInfo {
			rxBytes += ni.BytesRecv
		}
		return float64(rxBytes) / 1024
	case pm.Cyc:
		return cpushare.GetProcCycles(provideCpuPerc)
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
  Cpu - total CPU time (msec) spent on running process
  CpuPerc - percentage of the CPU usage by the process (%)
  Mem - process memory usage (KB)
  PIDs - number of process threads
  CPUs - number of host processors available to the process
  Rx - total network read bytes (KB)
  Tx - total network write bytes (KB)
  Cyc - total CPU cycles for the process (AMD64 and PPC64 only)`)
}
