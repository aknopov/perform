package cpushare

import (
	"testing"

	"github.com/aknopov/perform/mocker"
	"github.com/stretchr/testify/assert"
)

var (
	cpuPercInvocations = 0
	cyclesInvocations  = 0
	ticks              = []uint64{1000000, 2000000}
)

func mockCPUshare() float64 {
	cpuPercInvocations++
	return 55
}

func mockTickCount() uint64 {
	val := ticks[cyclesInvocations]
	cyclesInvocations++
	return val
}

func TestGetCpuPerc(t *testing.T) {
	assertT := assert.New(t)

	defer mocker.ReplaceItem(&lastCpuTime, NO_TIME)()

	// The first call reads the value
	assertT.EqualValues(55, GetCpuPerc(mockCPUshare))
	assertT.Equal(1, cpuPercInvocations)
	// The second call uses the same value
	assertT.EqualValues(55, GetCpuPerc(mockCPUshare))
	assertT.Equal(1, cpuPercInvocations)
}

func TestGetProcCycles(t *testing.T) {
	assertT := assert.New(t)

	defer mocker.ReplaceItem(&tickCountF, mockTickCount)()
	defer mocker.ReplaceItem(&prevTickCnt, 0)()
	defer mocker.ReplaceItem(&lastCpuTime, NO_TIME)()

	assertT.EqualValues(550000, GetProcCycles(mockCPUshare))
	assertT.EqualValues(1100000, GetProcCycles(mockCPUshare))
}
