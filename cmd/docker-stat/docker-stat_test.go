package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aknopov/perform/cmd/param"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/stretchr/testify/assert"
)

var (
	dockerInfo system.Info = system.Info{
		NCPU:     13,
		MemTotal: 768 * 1024 * 1024,
	}

	stats1 container.StatsResponse = container.StatsResponse{
		Read: time.Unix(100, 0),
		Networks: map[string]container.NetworkStats{
			"lo": {
				RxBytes: 63 * 1024,
				TxBytes: 287 * 1024,
			},
		},
		CPUStats: container.CPUStats{
			OnlineCPUs: 5,
			CPUUsage: container.CPUUsage{
				TotalUsage:        10000000000,
				UsageInUsermode:   20000000,
				UsageInKernelmode: 1000000,
			},
		},
		MemoryStats: container.MemoryStats{
			Usage: 3 * 1024 * 1024,
			Limit: 6 * 1024 * 1024,
		},
		PidsStats: container.PidsStats{
			Current: 12,
		},
	}

	stats2 container.StatsResponse = container.StatsResponse{
		Read: time.Unix(200, 0),
		Networks: map[string]container.NetworkStats{
			"eth0": {
				RxBytes: 125 * 1024,
				TxBytes: 522 * 1024,
			},
		},
		CPUStats: container.CPUStats{
			OnlineCPUs: 5,
			CPUUsage: container.CPUUsage{
				TotalUsage:        20000000000,
				UsageInUsermode:   30000000,
				UsageInKernelmode: 2000000,
			},
		},
		MemoryStats: container.MemoryStats{
			Usage: 3 * 1024 * 1024,
			Limit: 6 * 1024 * 1024,
		},
		PidsStats: container.PidsStats{
			Current: 12,
		},
	}
)

func TestAssertNoErrOnSuccess(t *testing.T) {
	assertT := assert.New(t)

	assertT.Equal(123, assertNoErr(123, nil))
}

func TestExitOnError(t *testing.T) {
	assertT := assert.New(t)

	// Testing exit code by starting external "go test""
	doTest := os.Getenv("DO_TEST")
	if doTest != "" {
		_ = assertNoErr("fail", fmt.Errorf("test error"))
		return // just in case
	}

	cmd := exec.Command(os.Args[0], "--test.run=TestExitOnError")
	cmd.Env = append(os.Environ(), "DO_TEST=yes")
	err := cmd.Run()
	e := err.(*exec.ExitError)
	assertT.Error(e)
	assertT.Equal(1, e.ExitCode())
}

func TestIsContainerAlive(t *testing.T) {
	assertT := assert.New(t)

	var stats container.StatsResponse

	stats.CPUStats.OnlineCPUs = 3
	assertT.True(isContainerAlive(&stats))

	stats.CPUStats.OnlineCPUs = 0
	assertT.False(isContainerAlive(&stats))
}

func TestCalcNetIO(t *testing.T) {
	assertT := assert.New(t)

	rxTotal, txTotal := calcNetIO(&stats1)
	assertT.EqualValues(63, rxTotal)
	assertT.EqualValues(287, txTotal)
}

func TestCalcCpu(t *testing.T) {
	assertT := assert.New(t)

	prevRead = NO_TIME

	// The first call yields zero
	assertT.Zero(calcCpuPerc(&stats1))
	// The second uses deltas and CPU count
	assertT.EqualValues(0.55, calcCpuPerc(&stats2))
}

func TestGetValue(t *testing.T) {
	assertT := assert.New(t)

	prevTotal = 0
	prevUser = 0
	prevKernel = 0
	prevRead = NO_TIME

	assertT.EqualValues(5, getValue(&dockerInfo, &stats1, param.CPUs))
	assertT.EqualValues(12, getValue(&dockerInfo, &stats1, param.PIDs))
	assertT.EqualValues(21, getValue(&dockerInfo, &stats1, param.Cpu))
	assertT.EqualValues(21, getValue(&dockerInfo, &stats1, param.Cpu))
	assertT.EqualValues(21, getValue(&dockerInfo, &stats1, param.Cpu))
	getValue(&dockerInfo, &stats1, param.Rx)
	assertT.EqualValues(125, getValue(&dockerInfo, &stats2, param.Rx))
	getValue(&dockerInfo, &stats1, param.Tx)
	assertT.EqualValues(522, getValue(&dockerInfo, &stats2, param.Tx))
	getValue(&dockerInfo, &stats1, param.CpuPerc)
	assertT.EqualValues(0.55, getValue(&dockerInfo, &stats2, param.CpuPerc))

	assertT.Panics(func() { getValue(&dockerInfo, &stats1, param.Tx+100) })
}

// https://vektra.github.io/mockery/latest/#why-mockery

func TestGetContainerInfo(t *testing.T) {
	assertT := assert.New(t)

	mockReader := container.StatsResponseReader{
		Body:   io.NopCloser(strings.NewReader(`{"Read": "2021-02-18T21:54:42Z"}`)),
		OSType: "Linux",
	}
	mockApiClient := NewMockContainerAPIClient(t)
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(mockReader, nil).Once()

	stats, err := getContainerInfo(mockApiClient, "ID")
	assertT.Nil(err)
	readTime, _ := time.Parse("2006-01-02T15:04:05Z", "2021-02-18T21:54:42Z")
	assertT.Equal(readTime, stats.Read)
}

func TestGetContainerInfoFailures(t *testing.T) {
	assertT := assert.New(t)

	dockerErr := errors.New("Docker is dead")
	mockApiClient := NewMockContainerAPIClient(t)
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(container.StatsResponseReader{}, dockerErr).Once()

	_, err := getContainerInfo(mockApiClient, "ID")
	assertT.Equal(dockerErr, err)

	mockReader := container.StatsResponseReader{
		Body:   io.NopCloser(strings.NewReader(`{"Read": "not a time"}`)),
		OSType: "Linux",
	}
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(mockReader, nil).Once()

	_, err = getContainerInfo(mockApiClient, "ID")
	assertT.IsType(&time.ParseError{}, err)
}

func TestPollStats(t *testing.T) {
	resp1 := container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{"read": "2021-02-18T21:54:42Z","cpu_stats": {"online_cpus": 1}}`))}
	resp2 := container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{"read": "2021-02-18T21:54:43Z","cpu_stats": {"online_cpus": 1}}`))}
	resp3 := container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(`{"read": "2021-02-18T21:54:44Z","cpu_stats": {"online_cpus": 0}}`))}

	mockApiClient := NewMockContainerAPIClient(t)
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(resp1, nil).Once()
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(resp2, nil).Once()
	mockApiClient.EXPECT().ContainerStatsOneShot(context.Background(), "ID").Return(resp3, nil).Once()

	pollStats(param.ParamList{param.Cpu, param.Mem}, 20*time.Millisecond, mockApiClient, &dockerInfo, "ID")
}

func TestUsagePrintout(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := param.CreateStream()

	usage(stream)

	output := param.ReadStream(stream, ch)
	assertT.True(strings.HasPrefix(output, "Docker container performance statistics\nUsage: docker-stat -refresh=... -params=... containerId"))
}
