package main

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aknopov/perform/monitor"
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
				RxBytes: 123 * 1024,
				TxBytes: 512 * 1024,
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

func TestAssertNoErr(t *testing.T) {
	assertT := assert.New(t)

	assertT.NotPanics(func() { assertNoErr(nil, "No error expected") })

	err := errors.New("Here you are")
	assertT.Panics(func() { assertNoErr(err, "Explanation") })
}

func TestIsContainerAlive(t *testing.T) {
	assertT := assert.New(t)

	var stats container.StatsResponse

	stats.CPUStats.OnlineCPUs = 3
	assertT.True(isContainerAlive(&stats))

	stats.CPUStats.OnlineCPUs = 0
	assertT.False(isContainerAlive(&stats))
}

func TestPrintHeader(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := monitor.CreateStream()

	var paramList monitor.ParamList = monitor.ParamList{monitor.CPUs, monitor.Tx}
	monitor.PrintHeader(stream, &paramList)

	output := monitor.ReadStream(stream, ch)
	assertT.Equal("Time                              CPUs    Tx MBps\n", output)

}

func TestPrintValues(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := monitor.CreateStream()

	monitor.PrintValues(stream, []float64{1.0, 13.0})

	output := monitor.ReadStream(stream, ch)
	tsRex := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} .*`)
	assertT.True(tsRex.MatchString(output))
	assertT.True(strings.HasSuffix(output, "         1.00       13.00\n"))
}

func TestCalcNetIO(t *testing.T) {
	assertT := assert.New(t)

	// The first call yeilds zeroes
	rxRate, txRate := calcNetIO(&stats1)
	assertT.Zero(rxRate)
	assertT.Zero(txRate)

	rxRate, txRate = calcNetIO(&stats2)
	assertT.Equal(0.02, rxRate)
	assertT.Equal(0.1, txRate)
}

func TestCalcCpu(t *testing.T) {
	assertT := assert.New(t)

	// The first call yeilds zero
	assertT.Zero(calcCpu(&stats1))
	// The second uses deltas and CPU count
	assertT.Equal(float32(0.55), calcCpu(&stats2))
}

func TestGetValue(t *testing.T) {
	assertT := assert.New(t)

	prevTotal = 0
	prevUser = 0
	prevKernel = 0
	prevTime = time.Time{}
	prevRx = 0
	prevTx = 0

	assertT.Equal(5.0, getValue(&dockerInfo, &stats1, monitor.CPUs))
	assertT.Equal(12.0, getValue(&dockerInfo, &stats1, monitor.PIDs))
	assertT.Equal(21.0, getValue(&dockerInfo, &stats1, monitor.Cpu))
	assertT.Equal(21.0, getValue(&dockerInfo, &stats1, monitor.Cpu))
	assertT.Equal(21.0, getValue(&dockerInfo, &stats1, monitor.Cpu))
	getValue(&dockerInfo, &stats1, monitor.Rx)
	assertT.Equal(0.02, getValue(&dockerInfo, &stats2, monitor.Rx))
	getValue(&dockerInfo, &stats1, monitor.Tx)
	assertT.Equal(0.1, getValue(&dockerInfo, &stats2, monitor.Tx))

	assertT.Panics(func() { getValue(&dockerInfo, &stats1, monitor.Tx+100) })
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

	// pollStats(&monitor.ParamList{monitor.CpuPerc, monitor.MemPerc}, 20*time.Millisecond, mockApiClient, &dockerInfo, "ID")
	pollStats(&monitor.ParamList{monitor.Cpu, monitor.Mem}, 20*time.Millisecond, mockApiClient, &dockerInfo, "ID")
}

func TestUsagePrintout(t *testing.T) {
	assertT := assert.New(t)

	stream, ch := monitor.CreateStream()

	usage(stream)

	output := monitor.ReadStream(stream, ch)
	assertT.True(strings.HasPrefix(output, "Docker container performance statistics\nUsage: docker-stat -refresh=... -params=... containerId"))
}
