package perform

import (
	"math"
	"math/big"
	"sort"
	"sync"
	"time"
)

// Statistics for running one task
type RunStats struct {
	Count     int
	TotalTime time.Duration
	AvgTime   time.Duration
	MinTime   time.Duration
	MaxTime   time.Duration
	MedTime   time.Duration
	StdDev    time.Duration
	Fails     int
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
//   - totalRuns - total number of tasks to run (> len(tasks))
//   - concurrent - number of concurrent tasks (< totalRuns)
//   - return time statistics for each task
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
		sorttimes := make([]time.Duration, len(fixture.runtimes))
		copy(sorttimes, fixture.runtimes)
		sort.Slice(sorttimes, func(i, j int) bool { return sorttimes[i] < sorttimes[j] })

		var sum = new(big.Float)
		var sum2 = new(big.Float)
		for _, t := range sorttimes {
			bigT := big.NewFloat(float64(t))
			sum = sum.Add(sum, bigT)
			sum2 = sum2.Add(sum2, bigT.Mul(bigT, bigT))
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

		ret = append(ret, testStats)
	}
	return ret
}

func big2float(val *big.Float) float64 {
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
