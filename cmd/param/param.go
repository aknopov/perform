package param

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"strings"
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
		Rx:   "Rx (KB)",
		Tx:   "Tx (KB)",
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
func ParseParams(args []string, usage func()) (string, *ParamList, float64, error) {
	progName := filepath.Base(args[0])
	flagSet := flag.NewFlagSet(progName, flag.ContinueOnError)
	flagSet.Usage = usage

	var paramList ParamList
	var refreshSec float64
	flagSet.Float64Var(&refreshSec, "refresh", 1.0, "")
	flagSet.Func("params", "", func(f string) error { return parseParamList(f, &paramList) })

	err := flagSet.Parse(args[1:])
	if err != nil {
		return "", nil, 0, err
	}

	otherArgs := flagSet.Args()
	if len(otherArgs) < 1 {
		return "", nil, 0, errors.New("container/process ID is missing")
	}

	return otherArgs[0], &paramList, refreshSec, nil
}

// Prints headers for monitored parameters
//
//nolint:errcheck
func PrintHeader(sink *os.File, paramList *ParamList) {
	fmt.Fprint(sink, "Time                   ")
	for _, p := range *paramList {
		fmt.Fprintf(sink, " %*s", colWidth, nameMap[p])
	}
	fmt.Fprintln(sink)
}

// Prints values of monitored parameters
//
//nolint:errcheck
func PrintValues(sink *os.File, values []float64) {
	fmt.Fprint(sink, time.Now().Format("2006-01-02 15:04:05.000"))
	for _, v := range values {
		fmt.Fprintf(sink, " %*.2f", colWidth, v)
	}
	fmt.Fprintln(sink)
}
