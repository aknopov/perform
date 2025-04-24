package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aknopov/perform/monitor"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

func main() {
	containerId, paramList, refreshSec, err := monitor.ParseParams(os.Args, func() { usage(os.Stderr) })
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
		usage(os.Stderr)
		os.Exit(1)
	}
	refreshPeriod := time.Duration(int64(refreshSec * float64(time.Second)))

	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.47"))
	assertNoErr(err, "Docker API not available")
	dockerInfo, err := apiClient.Info(context.Background())
	assertNoErr(err, "Failed to get Docker info - is daemon running?")

	monitor.PrintHeader(os.Stdout, paramList)
	pollStats(paramList, refreshPeriod, apiClient, &dockerInfo, containerId)
}

func pollStats(paramList *monitor.ParamList, refreshPeriod time.Duration, apiClient client.ContainerAPIClient, dockerInfo *system.Info, containerId string) {

	values := make([]float64, len(*paramList))
	ticker := time.NewTicker(refreshPeriod)
	for range ticker.C {
		stats, err := getContainerInfo(apiClient, containerId)
		assertNoErr(err, "Fail to get container info")
		if !isContainerAlive(stats) {
			break
		}

		for i, p := range *paramList {
			values[i] = getValue(dockerInfo, stats, p)
		}

		monitor.PrintValues(os.Stdout, values)
	}
}

func assertNoErr(err error, message string) {
	if err != nil {
		panic(fmt.Sprintf("%s:\n%s", message, err))
	}
}

func getContainerInfo(apiClient client.ContainerAPIClient, containerId string) (*container.StatsResponse, error) {
	resp, err := apiClient.ContainerStatsOneShot(context.Background(), containerId)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// Cheap and cheerful
func isContainerAlive(stats *container.StatsResponse) bool {
	return stats.CPUStats.OnlineCPUs != 0
}

func getValue(dockerInfo *system.Info, stats *container.StatsResponse, param monitor.ParamType) float64 {
	switch param {
	case monitor.CPUs:
		return float64(stats.CPUStats.OnlineCPUs)
	case monitor.Mem:
		return float64(stats.MemoryStats.Usage / 1024)
	case monitor.PIDs:
		return float64(stats.PidsStats.Current)
	case monitor.Rx:
		rx, _ := calcNetIO(stats)
		return rx
	case monitor.Tx:
		_, tx := calcNetIO(stats)
		return tx
	case monitor.Cpu:
		return float64((stats.CPUStats.CPUUsage.UsageInUsermode + stats.CPUStats.CPUUsage.UsageInKernelmode) / uint64(time.Millisecond))
	default:
		panic("Parameter value " + strconv.Itoa(int(param)) + " not recognised")
	}
}

var (
	// PreCPUStats is empty?!!
	prevTotal  uint64
	prevUser   uint64
	prevKernel uint64
	prevTime   time.Time
	prevRx     uint64
	prevTx     uint64
)

func calcCpu(stats *container.StatsResponse) float32 {
	totalDelta := stats.CPUStats.CPUUsage.TotalUsage - prevTotal
	userDelta := stats.CPUStats.CPUUsage.UsageInUsermode - prevUser
	kernelDelta := stats.CPUStats.CPUUsage.UsageInKernelmode - prevKernel

	prevTotal = stats.CPUStats.CPUUsage.TotalUsage
	prevUser = stats.CPUStats.CPUUsage.UsageInUsermode
	prevKernel = stats.CPUStats.CPUUsage.UsageInKernelmode

	if prevTotal == totalDelta || totalDelta == 0.0 {
		return 0.0
	}
	return float32(float64(userDelta+kernelDelta)/float64(totalDelta)) * 100.0 * float32(stats.CPUStats.OnlineCPUs)
}

func calcNetIO(stats *container.StatsResponse) (float64, float64) {
	var rxTotal uint64 = 0
	var txTotal uint64 = 0
	for _, ns := range stats.Networks {
		rxTotal += ns.RxBytes
		txTotal += ns.TxBytes
	}
	rxDelta := rxTotal - prevRx
	txDelta := txTotal - prevTx
	timeDelta := stats.Read.Sub(prevTime).Nanoseconds()

	defer func() { prevTime = stats.Read }()
	prevRx = rxTotal
	prevTx = txTotal
	if prevTime.IsZero() || timeDelta == 0 {
		return 0.0, 0.0
	}

	deltaSec := float64(timeDelta) / float64(time.Second)
	return float64(rxDelta) / deltaSec / 1024, float64(txDelta) / deltaSec / 1024
}

func usage(sink *os.File) {
	fmt.Fprintln(sink, `Docker container performance statistics
Usage: docker-stat -refresh=... -params=... containerId
  containerId - container name or ID
-refresh - interval in seconds (default 1.0 sec)
-params - comma separated list of
  Cpu - total CPU time (msec) spent on runing container
  Mem - container memory usage (KB)
  PIDs - number of container threads
  CPUs - number of processors available to the container
  Rx - total network read rate (KBs)
  Tx - total network write rate (KBs)`)
}
