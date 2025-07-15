package perform

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"
)

// Statistics for running one task - time values in milliseconds
type RunStats struct {
	Count   int       `json,yaml:"count"`
	AvgTime float64   `json,yaml:"avg_time"`
	MinTime float64   `json,yaml:"min_time"`
	MaxTime float64   `json,yaml:"max_time"`
	MedTime float64   `json,yaml:"med_time"`
	StdDev  float64   `json,yaml:"stdev_time"`
	Fails   int       `json,yaml:"fails"`
	Values  []float64 `json,yaml:"times"`
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

const msecFctr = float64(time.Millisecond)

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
			return nil, fmt.Errorf("invalid statistics data in test #%d: %v", i, err)
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

	for _, fixture := range fixtures {
		var testStats RunStats
		testStats.Values = make([]float64, len(fixture.runtimes))
		for i, t := range fixture.runtimes {
			testStats.Values[i] = float64(t) / msecFctr
		}

		sorttimes := make([]float64, len(fixture.runtimes))
		copy(sorttimes, testStats.Values)
		sort.Slice(sorttimes, func(i, j int) bool { return sorttimes[i] < sorttimes[j] })

		testCount := len(sorttimes)
		testStats.Count = len(sorttimes)
		testStats.Fails = fixture.fails
		testStats.AvgTime = mean(sorttimes)
		testStats.MinTime = sorttimes[0]
		testStats.MedTime = sorttimes[testCount/2]
		testStats.MaxTime = sorttimes[testCount-1]
		testStats.StdDev = math.Sqrt(variance(sorttimes))

		ret = append(ret, testStats)
	}
	return ret
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
		fmt.Fprintf(os.Stderr, "\x1b[35m%v\x1b[0m\n", err)
		// println(err.Error())
		return defVal
	}
	return val
}
