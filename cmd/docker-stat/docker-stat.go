package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aknopov/perform/cmd/param"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

var (
	NO_STAT = &container.StatsResponse{}
	NO_TIME = time.Time{}
)

func main() {
	containerId, paramList, refreshSec, err := param.ParseParams(os.Args, func() { usage(os.Stderr) })
	if err != nil {
		if err.Error() != "flag: help requested" {
			fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
			usage(os.Stderr)
		}
		os.Exit(1)
	}
	refreshPeriod := time.Duration(int64(refreshSec * float64(time.Second)))

	apiClient := assertNoErr(client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.47")))
	dockerInfo := assertNoErr(apiClient.Info(context.Background()))

	param.PrintHeader(os.Stdout, paramList)
	pollStats(paramList, refreshPeriod, apiClient, &dockerInfo, containerId)
}

func pollStats(paramList param.ParamList, refreshPeriod time.Duration, apiClient client.ContainerAPIClient, dockerInfo *system.Info, containerId string) {

	values := make([]float64, len(paramList))
	ticker := time.NewTicker(refreshPeriod)
	for range ticker.C {
		stats := assertNoErr(getContainerInfo(apiClient, containerId))
		if !isContainerAlive(stats) {
			break
		}

		for i, p := range paramList {
			values[i] = getValue(dockerInfo, stats, p)
		}

		param.PrintValues(os.Stdout, paramList, values)
	}
}

func assertNoErr[T any](val T, err error) T {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return val
}

func getContainerInfo(apiClient client.ContainerAPIClient, containerId string) (*container.StatsResponse, error) {
	resp, err := apiClient.ContainerStatsOneShot(context.Background(), containerId)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

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

func getValue(dockerInfo *system.Info, stats *container.StatsResponse, p param.ParamType) float64 {
	switch p {
	case param.CPUs:
		return float64(stats.CPUStats.OnlineCPUs)
	case param.Mem:
		return float64(stats.MemoryStats.Usage / 1024)
	case param.PIDs:
		return float64(stats.PidsStats.Current)
	case param.Rx:
		rx, _ := calcNetIO(stats)
		return rx
	case param.Tx:
		_, tx := calcNetIO(stats)
		return tx
	case param.Cpu:
		return float64((stats.CPUStats.CPUUsage.UsageInUsermode + stats.CPUStats.CPUUsage.UsageInKernelmode) / uint64(time.Millisecond))
	case param.CpuPerc:
		return calcCpuPerc(stats)
	default:
		panic(fmt.Errorf("unknown parameter type: %v", p))
	}
}

var (
	// PreCPUStats and stats.PreRead are empty?!!
	prevTotal  uint64
	prevUser   uint64
	prevKernel uint64
	prevRead   time.Time
)

func calcCpuPerc(stats *container.StatsResponse) float64 {
	totalDelta := stats.CPUStats.CPUUsage.TotalUsage - prevTotal
	userDelta := stats.CPUStats.CPUUsage.UsageInUsermode - prevUser
	kernelDelta := stats.CPUStats.CPUUsage.UsageInKernelmode - prevKernel

	prevTotal = stats.CPUStats.CPUUsage.TotalUsage
	prevUser = stats.CPUStats.CPUUsage.UsageInUsermode
	prevKernel = stats.CPUStats.CPUUsage.UsageInKernelmode
	defer func() { prevRead = stats.Read }()

	if prevRead.IsZero() || totalDelta == 0.0 {
		return 0.0
	}

	return float64(userDelta+kernelDelta) / float64(totalDelta) * 100.0 * float64(stats.CPUStats.OnlineCPUs)
}

func calcNetIO(stats *container.StatsResponse) (float64, float64) {
	var rxTotal uint64 = 0
	var txTotal uint64 = 0
	for _, ns := range stats.Networks {
		rxTotal += ns.RxBytes
		txTotal += ns.TxBytes
	}

	return float64(rxTotal) / 1024, float64(txTotal) / 1024
}

//nolint:errcheck
func usage(sink *os.File) {
	fmt.Fprintln(sink, `Docker container performance statistics
Usage: docker-stat -refresh=... -params=... containerId
  containerId - container name or ID
-refresh - interval in seconds (default 1.0 sec)
-params - comma separated list of
  Cpu - total CPU time (msec) spent on runing container
  CpuPerc - percentage of the host's CPU usage
  Mem - container memory usage (KB)
  PIDs - number of container threads
  CPUs - number of processors available to the container
  Rx - total network read bytes (KB)
  Tx - total network write bytes (KB)`)
}
