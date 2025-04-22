package monitor

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"strings"

	"github.com/aknopov/perform"
)

type ParamType int
type ParamList []ParamType

const (
	// CPU time (user + kernel) in milliseconds
	Cpu ParamType = iota
	// Memory use in kilobytes
	Mem
	// Number of Docker "PIDs"
	PIDs
	// Number of available CPU's to a container
	CPUs
	// Received data rate in KB/sec
	Rx
	// Transmitted data rate in KB/sec
	Tx

	paramFirst = Cpu
	paramLast  = Tx
)

var (
	convertMap = map[string]ParamType{
		"Cpu":  Cpu,
		"Mem":  Mem,
		"PIDs": PIDs,
		"CPUs": CPUs,
		"Rx":   Rx,
		"Tx":   Tx,
	}

	nameMap = map[ParamType]string{
		Cpu:  "CPU (ms)",
		Mem:  "Mem (KB)",
		PIDs: "PIDs",
		CPUs: "CPUs",
		Rx:   "Rx MBps",
		Tx:   "Tx MBps",
	}
)

const (
	colWidth = 11
)

func parseParamList(flagValues string, paramList *ParamList) error {
	for _, val := range strings.Split(flagValues, ",") {
		if param, ok := convertMap[strings.TrimSpace(val)]; ok {
			*paramList = append(*paramList, param)
		}
	}
	return nil
}

// Parses commandline; returns program name,  monitored parameters list, monitoring frequency
func ParseParams(flagSet *flag.FlagSet, args []string) (string, *ParamList, float64, error) {
	var paramList ParamList
	var refreshSec float64
	flagSet.Usage = func() {
		fmt.Fprintln(os.Stderr, "Logs Docker container statistics")
		fmt.Fprintf(os.Stderr, "Usage: %s -refresh=... -params=... containerId\n", flagSet.Name())
		fmt.Fprintln(os.Stderr, "  containerId - container name or ID")
		fmt.Fprintln(os.Stderr, "  -refresh - interval in seconds (default 1.0 sec)")
		fmt.Fprintln(os.Stderr, "  -params - comma separated list of")
		fmt.Fprintln(os.Stderr, "    Cpu - total CPU time spent on runing container")
		fmt.Fprintln(os.Stderr, "    Mem - process memory")
		fmt.Fprintln(os.Stderr, "    PIDs - number of container threads")
		fmt.Fprintln(os.Stderr, "    CPUs - number of host processors available to container")
		fmt.Fprintln(os.Stderr, "    Rx - total network read rate (KBs)")
		fmt.Fprintln(os.Stderr, "    Tx - total network write rate (KBs)")
	}
	flagSet.Float64Var(&refreshSec, "refresh", 1.0, "")
	flagSet.Func("params", "", func(f string) error { return parseParamList(f, &paramList) })
	perform.AssertNoErr(perform.ND, flagSet.Parse(args))

	otherArgs := flagSet.Args()
	if len(otherArgs) < 1 {
		fmt.Fprintln(os.Stderr)
		flagSet.Usage()
		return "", nil, 0, errors.New("container ID is missing")
	}

	return otherArgs[0], &paramList, refreshSec, nil
}

// Prints headers for monitored parameters
func PrintHeader(sink *os.File, paramList *ParamList) {
	fmt.Fprint(sink, "Time                       ")
	for _, p := range *paramList {
		fmt.Fprintf(sink, "%*s", colWidth, nameMap[p])
	}
	fmt.Fprintln(sink)
}

// Prints values of monitored parameters
func PrintValues(sink *os.File, values []float32) {
	fmt.Fprint(sink, time.Now().Format("2006-01-02 15:04:05.000  "))
	for _, v := range values {
		fmt.Fprintf(sink, " %*.2f", colWidth, v)
	}
	fmt.Fprintln(sink)
}
