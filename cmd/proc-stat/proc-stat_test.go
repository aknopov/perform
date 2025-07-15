package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	pm "github.com/aknopov/perform/cmd/param"
	"github.com/aknopov/perform/cmd/proc-stat/net"
	"github.com/aknopov/perform/mocker"
	ps "github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
)

var (
	testTimes  = cpu.TimesStat{User: 1234, System: 555}
	testMemory = process.MemoryInfoStat{RSS: 1024 * 1024}
	testNetIO  = net.IOCountersStat{BytesSent: 2 * 1024, BytesRecv: 3 * 1024}
	errTest    = fmt.Errorf("test error")
)

func TestGetProcPid(t *testing.T) {
	assertT := assert.New(t)

	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)
	mockProcess.EXPECT().Executable().Return("prog.out")

	testGetProcessList := func() ([]ps.Process, error) { return []ps.Process{mockProcess}, nil }
	defer mocker.ReplaceItem(&getProcessList, testGetProcessList)()

	pid, cmd := getProcIds("prog")
	assertT.Equal(123, pid)
	assertT.Equal("prog.out", cmd)
	pid, cmd = getProcIds("123")
	assertT.Equal(123, pid)
	assertT.Equal("prog.out", cmd)
}

func TestGetProcIds(t *testing.T) {
	assertT := assert.New(t)

	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)
	mockProcess.EXPECT().Executable().Return("prog")

	testGetProcessList := func() ([]ps.Process, error) { return []ps.Process{mockProcess}, nil }
	defer mocker.ReplaceItem(&getProcessList, testGetProcessList)()

	pid, app := getProcIds("prog")
	assertT.Equal(123, pid)
	assertT.Equal("prog", app)
	pid, app = getProcIds("123")
	assertT.Equal(123, pid)
	assertT.Equal("prog", app)

	pid, app = getProcIds("321")
	assertT.Equal(-1, pid)
	assertT.Equal("", app)
	pid, app = getProcIds("boo")
	assertT.Equal(-1, pid)
	assertT.Equal("", app)
}

func TestExitOnNoProcess(t *testing.T) {
	assertT := assert.New(t)

	// Testing exit code by starting external "go test""
	testPid := os.Getenv("TEST_PID")
	if testPid != "" {
		main()
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

	testFindProcess := func(pid int) (ps.Process, error) { return mockProcess, nil }
	defer mocker.ReplaceItem(&findProcess, testFindProcess)()

	assertT.True(isProcessAlive(123))
}

func TestIsProcessAliveFail(t *testing.T) {
	assertT := assert.New(t)

	testFindProcess := func(pid int) (ps.Process, error) { return nil, errTest }
	defer mocker.ReplaceItem(&findProcess, testFindProcess)()

	assertT.False(isProcessAlive(123))

	findProcess = func(pid int) (ps.Process, error) {
		return nil, nil
	}

	assertT.False(isProcessAlive(123))
}
func TestNetIO(t *testing.T) {
	assertT := assert.New(t)

	qProc := NewMockQIQProcess(t)

	assertT.EqualValues(2, getValue(qProc, &testNetIO, pm.Tx))
	assertT.EqualValues(3, getValue(qProc, &testNetIO, pm.Rx))
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
	defer mocker.ReplaceItem(&findProcess, testFindProcess)()

	paramList := pm.ParamList{pm.Cpu}

	pollStats(qProc, paramList, 100*time.Millisecond)
}

func TestGetValue(t *testing.T) {
	assertT := assert.New(t)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().Times().Return(&testTimes, nil)
	qProc.EXPECT().MemoryInfo().Return(&testMemory, nil)
	qProc.EXPECT().NumThreads().Return(13, nil)
	qProc.EXPECT().Percent(time.Duration(0)).Return(44, nil)

	newGetNumCPU := func() int { return 1 }
	defer mocker.ReplaceItem(&getNumCPU, newGetNumCPU)()

	assertT.EqualValues(1234+555, getValue(qProc, &testNetIO, pm.Cpu))
	assertT.EqualValues(1024, getValue(qProc, &testNetIO, pm.Mem))
	assertT.EqualValues(1, getValue(qProc, &testNetIO, pm.CPUs))
	assertT.EqualValues(13, getValue(qProc, &testNetIO, pm.PIDs))
	// measurement starts from 0 - see TestNetIO
	assertT.EqualValues(2, getValue(qProc, &testNetIO, pm.Tx))
	assertT.EqualValues(3, getValue(qProc, &testNetIO, pm.Rx))
	assertT.EqualValues(44, getValue(qProc, &testNetIO, pm.CpuPerc))

	assertT.Panics(func() { getValue(qProc, &testNetIO, pm.Rx+100) })
}

func TestPollCyclesStats(t *testing.T) {
	mockProcess := NewMockPsProcess(t)
	mockProcess.EXPECT().Pid().Return(123)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().GetPID().Return(123).Times(2)
	qProc.EXPECT().Percent(time.Duration(0)).Return(90, nil).Once()

	cnt := 0
	retVals := []struct {
		p ps.Process
		e error
	}{{mockProcess, nil}, {nil, errTest}}
	testFindProcess := func(pid int) (ps.Process, error) {
		ret := retVals[cnt]
		cnt++
		return ret.p, ret.e
	}
	defer mocker.ReplaceItem(&findProcess, testFindProcess)()

	paramList := pm.ParamList{pm.Cyc}
	pollStats(qProc, paramList, 100*time.Millisecond)
}

func TestGetValueRecovery(t *testing.T) {
	assertT := assert.New(t)

	qProc := NewMockQIQProcess(t)
	qProc.EXPECT().Times().Return(nil, errTest).Once()
	qProc.EXPECT().MemoryInfo().Return(nil, errTest).Once()
	qProc.EXPECT().NumThreads().Return(0, errTest)

	assertT.EqualValues(0, getValue(qProc, &testNetIO, pm.Cpu))
	assertT.EqualValues(0, getValue(qProc, &testNetIO, pm.Mem))
	assertT.EqualValues(-1, getValue(qProc, &testNetIO, pm.PIDs))
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
