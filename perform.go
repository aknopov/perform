package perform

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/ericlagergren/decimal"
)

// Statistics for running one task
type RunStats struct {
	Count     int             `json,yaml:"count"`
	TotalTime time.Duration   `json,yaml:"sum_time"`
	AvgTime   time.Duration   `json,yaml:"avg_time"`
	MinTime   time.Duration   `json,yaml:"min_time"`
	MaxTime   time.Duration   `json,yaml:"max_time"`
	MedTime   time.Duration   `json,yaml:"med_time"`
	StdDev    time.Duration   `json,yaml:"stdev_time"`
	Fails     int             `json,yaml:"fails"`
	Values    []time.Duration `json,yaml:"times"`
}

// Generic test task
type TestTask func() error

type taskFixture struct {
	sema      *chan struct{}  // threads number throttle - shared
	waitGroup *sync.WaitGroup // completion flag - shared
	lock      sync.Mutex      // `runtimes` guard
	task      TestTask
	runtimes  []time.Duration
	fails     int
}

// No data struct
var ND = struct{}{}

// Runs concurrently several tasks
//
//   - tasks - tasks to run
//
//   - totalRuns - total number of tasks to run (> len(tasks))
//
//   - concurrent - number of concurrent tasks (< totalRuns)
//
//     return time statistics for each task
func RunTest(tasks []TestTask, totalRuns int, concurrent int) []RunStats {
	waitGroup := new(sync.WaitGroup)
	sema := make(chan struct{}, concurrent)
	fixtures := make([]*taskFixture, len(tasks))
	for i, task := range tasks {
		fixtures[i] = createFixture(task, &sema, waitGroup)
	}

	for i := 0; i < totalRuns; i++ {
		idx := i % len(tasks)
		waitGroup.Add(1)
		go runOneTask(fixtures[idx])
	}

	waitGroup.Wait()

	return calcStats(fixtures)
}

// Compares two series of tests and calculates probabilities that latencies in the second series
// are larger using t-test statistics.
//
// Statistics "stat1" and "stats2" should have same number od tests; run counts in test pairs are not required to be equal,
// but they should be larger than 1.
func CalcPvals(stats1, stats2 []RunStats) ([]float64, error) {
	if len(stats1) != len(stats2) {
		return nil, errors.New("different size of tasks")
	}

	pVals := make([]float64, 0, len(stats1))
	for i := range stats1 {
		tSample1 := timeStat2Tstat(stats1[i])
		tSample2 := timeStat2Tstat(stats2[i])

		tRes, err := TwoSampleTTest(tSample1, tSample2, LocationGreater)
		if err != nil {
			return nil, fmt.Errorf("invalid statistics data in sample %d: %v", i, err)
		}

		pVals = append(pVals, tRes.P)
	}

	return pVals, nil
}

func createFixture(task TestTask, sema *chan struct{}, waitGroup *sync.WaitGroup) *taskFixture {
	var fixture taskFixture
	fixture.sema = sema
	fixture.waitGroup = waitGroup
	fixture.lock = sync.Mutex{}
	fixture.task = task
	fixture.runtimes = make([]time.Duration, 0)
	return &fixture
}

func runOneTask(fixture *taskFixture) {
	*fixture.sema <- ND
	defer func() { <-*fixture.sema }()
	defer fixture.waitGroup.Done()

	start := time.Now()
	err := fixture.task()
	execTime := time.Since(start)

	fixture.lock.Lock()
	fixture.runtimes = append(fixture.runtimes, execTime)
	if err != nil {
		fixture.fails++
	}
	fixture.lock.Unlock()
}

func calcStats(fixtures []*taskFixture) []RunStats {
	ret := make([]RunStats, 0)

	precCtx := decimal.Context128
	for _, fixture := range fixtures {
		sorttimes := make([]time.Duration, len(fixture.runtimes))
		copy(sorttimes, fixture.runtimes)
		sort.Slice(sorttimes, func(i, j int) bool { return sorttimes[i] < sorttimes[j] })

		sum := new(decimal.Big)
		sum2 := new(decimal.Big)
		bigT := new(decimal.Big)
		for _, t := range sorttimes {
			bigT.SetUint64(uint64(t))
			precCtx.Add(sum, sum, bigT)
			precCtx.Add(sum2, sum2, bigT.Mul(bigT, bigT))
		}

		testCount := len(sorttimes)
		var testStats RunStats
		testStats.Count = testCount
		testStats.TotalTime = time.Duration(big2float(sum))
		testStats.AvgTime = time.Duration(big2float(sum) / float64(testCount))
		testStats.MinTime = sorttimes[0]
		testStats.MedTime = sorttimes[testCount/2]
		testStats.MaxTime = sorttimes[testCount-1]
		fCount := float64(testCount)
		testStats.StdDev = time.Duration(math.Sqrt(big2float(sum2)/(fCount-1) - big2float(sum)*big2float(sum)/fCount/(fCount-1)))

		testStats.Fails = fixture.fails

		testStats.Values = fixture.runtimes

		ret = append(ret, testStats)
	}
	return ret
}

func big2float(val *decimal.Big) float64 {
	conv, _ := val.Float64()
	return conv
}

// Aid fo unexpected errors without recovery
func AssertNoErr[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}
	return val
}

// Recover from error - assume default value
func AssumeOnErr[T any](f func() (T, error), defVal T) T {
	val, err := f()
	if err != nil {
		print(err.Error())
		return defVal
	}
	return val
}
