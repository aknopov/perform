// The package deals with CPU usage - CPU% and CPU cycles count for a process.
// Since both "CpuPerc" and "Cyc" can be requested, the package functions avoid double calculation of CPU%.
package cpushare

import (
	"time"

	"github.com/aknopov/perform/tickcount"
)

var (
	NO_TIME = time.Time{}
)

// Function type that provides share of CPU usage by a process as a percent (0 - 100)
type CPUPercFun func() float64

const (
	minRefreshIntvl = 10 * time.Millisecond
)

// Function substitutions for unit tests
var (
	tickCountF = tickcount.TickCount
)

var (
	prevTickCnt = tickCountF()
	cyclesTotal = 0.0
	lastCpuTime = NO_TIME
	cpuPercent  = 0.0
)

// Invokes "cpuProvider" if previous call was "long time ago" (10 msec).
// Otherwise provides previous CPU usage value.
func GetCpuPerc(cpuProvider CPUPercFun) float64 {
	currTime := time.Now()
	if currTime.Sub(lastCpuTime) > minRefreshIntvl {
		cpuPercent = cpuProvider()
		lastCpuTime = currTime
	}
	return cpuPercent
}

// Calculates cumulative number of CPU cycles used by a process.
// Function returns current value incremented by a product of CPU% and change of CPU cycles
func GetProcCycles(cpuProvider CPUPercFun) float64 {
	currTickCnt := tickCountF()
	delta := currTickCnt - prevTickCnt
	prevTickCnt = currTickCnt
	cyclesTotal += GetCpuPerc(cpuProvider) * float64(delta) / 100
	return cyclesTotal
}
