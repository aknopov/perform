package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	pm "github.com/aknopov/perform/cmd/param"
	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
)

var (
	testTimes  = cpu.TimesStat{User: 1234, System: 555}
	testMemory = process.MemoryInfoStat{RSS: 1024 * 1024}
	testNetIO  = []net.IOCountersStat{{BytesSent: 22222, BytesRecv: 33333}}
	errTest    = fmt.Errorf("test error")
)

func TestGetProcPid(t *testing.T) {
	assertT := assert.New(t)

	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)
	mockProcess.EXPECT().Executable().Return("prog")

	testGetProcessList := func() ([]ps.Process, error) {
		return []ps.Process{mockProcess}, nil
	}
	defer replaceFun0(&getProcessList, testGetProcessList)()

	pid := getProcPid("prog")
	assertT.Equal(123, pid)
	pid = getProcPid("123")
	assertT.Equal(123, pid)
}

func TestExitOnNoProcess(t *testing.T) {
	assertT := assert.New(t)

	// Testing exit code by starting external "go test""
	testPid := os.Getenv("TEST_PID")
	if testPid != "" {
		getProcPid(testPid)
		return // just in case
	}

	cmd := exec.Command(os.Args[0], "--test.run=TestExitOnNoProcess")
	cmd.Env = append(os.Environ(), "TEST_PID=boo")
	err := cmd.Run()
	e := err.(*exec.ExitError)
	assertT.Error(e)
	assertT.Equal(1, e.ExitCode())
}

func TestIsProcessAlive(t *testing.T) {
	assertT := assert.New(t)

	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)

	testFindProcess := func(pid int) (ps.Process, error) {
		return mockProcess, nil
	}
	defer replaceFun1(&findProcess, testFindProcess)()

	assertT.True(isProcessAlive(123))
}

func TestIsProcessAliveFail(t *testing.T) {
	assertT := assert.New(t)

	testFindProcess := func(pid int) (ps.Process, error) {
		return nil, errTest
	}
	defer replaceFun1(&findProcess, testFindProcess)()

	assertT.False(isProcessAlive(123))

	findProcess = func(pid int) (ps.Process, error) {
		return nil, nil
	}

	assertT.False(isProcessAlive(123))
}
func TestNetIO(t *testing.T) {
	assertT := assert.New(t)

	qProc := NewMockQIQProcess(t)
	testNetIO2 := []net.IOCountersStat{{BytesSent: 22222 + 2*1024, BytesRecv: 33333 + 3*1024}}

	prevRx = 0
	prevTx = 0
	// First values yield 0
	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Tx))
	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Rx))
	// Following values are based on the first
	assertT.EqualValues(2, getValue(qProc, testNetIO2, pm.Tx))
	assertT.EqualValues(3, getValue(qProc, testNetIO2, pm.Rx))
}

func TestPollStats(t *testing.T) {
	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().GetPID().Return(123).Times(2)
	qProc.EXPECT().Times().Return(&testTimes, nil).Once()

	cnt := 0
	testFindProcess := func(pid int) (ps.Process, error) {
		cnt++
		if cnt == 1 {
			return mockProcess, nil
		} else {
			return nil, errTest
		}
	}
	defer replaceFun1(&findProcess, testFindProcess)()

	paramList := pm.ParamList{pm.Cpu}

	pollStats(qProc, paramList, 100*time.Millisecond)
}

func TestPollStatsWithNet(t *testing.T) {
	prevRx = 0
	prevTx = 0
	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().GetPID().Return(123).Times(2)
	qProc.EXPECT().Times().Return(&testTimes, nil).Once()
	qProc.EXPECT().NetIOCounters(false).Return(testNetIO, nil).Once()

	cnt := 0
	testFindProcess := func(pid int) (ps.Process, error) {
		cnt++
		if cnt == 1 {
			return mockProcess, nil
		} else {
			return nil, errTest
		}
	}
	defer replaceFun1(&findProcess, testFindProcess)()

	paramList := pm.ParamList{pm.Cpu, pm.Tx}

	pollStats(qProc, paramList, 100*time.Millisecond)
}

func TestGetValue(t *testing.T) {
	assertT := assert.New(t)

	prevRx = 0
	prevTx = 0
	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().Times().Return(&testTimes, nil)
	qProc.EXPECT().MemoryInfo().Return(&testMemory, nil)
	qProc.EXPECT().NumThreads().Return(13, nil)

	newGetNumCPU := func() int { return 256 }
	defer replaceFunPlain(&getNumCPU, newGetNumCPU)()

	assertT.EqualValues(1234+555, getValue(qProc, testNetIO, pm.Cpu))
	assertT.EqualValues(1024, getValue(qProc, testNetIO, pm.Mem))
	assertT.EqualValues(256, getValue(qProc, testNetIO, pm.CPUs))
	assertT.EqualValues(13, getValue(qProc, testNetIO, pm.PIDs))
	// measurement starts from 0 - see TestNetIO
	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Tx))
	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Rx))
	assertT.Panics(func() { getValue(qProc, testNetIO, pm.Rx+100) })
}

func TestGetProcCycles(t *testing.T) {
	assertT := assert.New(t)

	mockTickCountF := func() uint64 { return 1000100 }
	replaceFunPlain(&tickCountF, mockTickCountF)
	tickOverhead = 100
	prevTickCnt = 0

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().Percent(time.Duration(0)).Return(90, nil).Once()

	assertT.EqualValues(900000, getProcCycles(qProc))
}

func TestPollCyclesStats(t *testing.T) {
	assertT := assert.New(t)

	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().GetPID().Return(123).Times(2)
	qProc.EXPECT().Percent(time.Duration(0)).Return(90, nil).Once()

	cnt := 0
	testFindProcess := func(pid int) (ps.Process, error) {
		cnt++
		if cnt == 1 {
			return mockProcess, nil
		} else {
			return nil, errTest
		}
	}
	defer replaceFun1(&findProcess, testFindProcess)()

	tiCountCalls := 0
	mockTickCountF := func() uint64 { tiCountCalls++; return 0 }
	replaceFunPlain(&tickCountF, mockTickCountF)

	paramList := pm.ParamList{pm.Cyc}
	pollStats(qProc, paramList, 100*time.Millisecond)

	assertT.Equal(1, tiCountCalls)
}

func TestGetValueRecovery(t *testing.T) {
	assertT := assert.New(t)

	prevRx = 0
	prevTx = 0
	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().Times().Return(nil, errTest).Once()
	qProc.EXPECT().MemoryInfo().Return(nil, errTest).Once()
	qProc.EXPECT().NumThreads().Return(0, errTest)

	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Cpu))
	assertT.EqualValues(0, getValue(qProc, testNetIO, pm.Mem))
	assertT.EqualValues(-1, getValue(qProc, testNetIO, pm.PIDs))
	assertT.EqualValues(0, getValue(qProc, NO_NET_IO, pm.Tx))
	assertT.EqualValues(0, getValue(qProc, NO_NET_IO, pm.Rx))
}

func TestUsage(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := pm.CreateStream()

	usage(stream)

	output := pm.ReadStream(stream, ch)
	assertT.True(strings.HasPrefix(output, "Application performance statistics\nUsage: proc-stat -refresh=... -params=... proc"))
}
