package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aknopov/perform"
	"github.com/aknopov/perform/cmd/cpushare"
	pm "github.com/aknopov/perform/cmd/param"
	"github.com/aknopov/perform/cmd/proc-stat/net"

	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

var (
	NO_TIMESTAT = &cpu.TimesStat{}
	NO_MEMSTAT  = &process.MemoryInfoStat{}
	NO_NET_IO   = &net.IOCountersStat{}
	ERROR_WAIT  = 100 * time.Millisecond
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

	pid, cmd := getProcIds(procId)
	if pid == -1 {
		fmt.Fprintf(os.Stderr, "Can't find process with PID or command line '%s'\n", procId)
		os.Exit(1)
	}

	p, _ := process.NewProcess(int32(pid))
	fmt.Printf("Getting performance data for the process '%s' (pid=%d)\n\n", cmd, pid)

	if slices.Contains(paramList, pm.Rx) || slices.Contains(paramList, pm.Tx) {
		errChan := net.StartTracing(context.Background(), p.Pid, refreshPeriod/2)
		go watchErrors(context.Background(), errChan, os.Stderr)
	}

	pm.PrintHeader(os.Stdout, paramList)
	pollStats(pm.NewQProcess(p), paramList, refreshPeriod)
}

func watchErrors(ctx context.Context, errChan chan error, sink io.Writer) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			fmt.Fprintf(sink, "\x1b[35m%v\x1b[0m\n", err) //nolint:errcheck
		default:
			time.Sleep(ERROR_WAIT)
		}
	}
}

func reduceArg[A any, R any](f func(A) (R, error), arg A) func() (R, error) {
	return func() (R, error) { return f(arg) }
}

func pollStats(proc pm.IQProcess, paramList pm.ParamList, refreshPeriod time.Duration) {

	queryNet := slices.Contains(paramList, pm.Rx) || slices.Contains(paramList, pm.Tx)
	var netStat *net.IOCountersStat

	ticker := time.NewTicker(refreshPeriod)
	values := make([]float64, len(paramList))

	for range ticker.C {
		if !isProcessAlive(int(proc.GetPID())) {
			fmt.Fprintln(os.Stderr, "\x1b[31mProcess terminated\x1b[0m")
			break
		}

		if queryNet {
			netStat = perform.IgnoreErr(net.GetProcessNetIOCounters, NO_NET_IO)
		}

		for i, p := range paramList {
			values[i] = getValue(proc, netStat, p)
		}

		pm.PrintValues(os.Stdout, paramList, values)
	}
}

func getValue(proc pm.IQProcess, netStat *net.IOCountersStat, p pm.ParamType) float64 {

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
		return float64(netStat.BytesSent) / 1024
	case pm.Rx:
		return float64(netStat.BytesRecv) / 1024
	case pm.Cyc:
		return cpushare.GetProcCycles(provideCpuPerc)
	default:
		panic(fmt.Errorf("unknown parameter type: %v", p))
	}
}

func getProcIds(cmd string) (int, string) {
	procList := perform.AssertNoErr(getProcessList())

	pid, err := strconv.Atoi(cmd)

	for _, p := range procList {
		if err == nil && pid == p.Pid() {
			return pid, p.Executable()
		} else if strings.Contains(p.Executable(), cmd) {
			return p.Pid(), p.Executable()
		}
	}

	return -1, ""
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
