package perform

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	TotalTests = 100
	Parallel   = 10
	SleepTime  = time.Duration(10) * time.Millisecond
)

var (
	waitGroup = sync.WaitGroup{}
	sema      = make(chan struct{}, 1)
	errTest   = errors.New("test error")
)

func TestCreateFixture(t *testing.T) {
	assertT := assert.New(t)

	var task TestTask = func() error { return nil }

	fixture := createFixture(task, &sema, &waitGroup)

	assertT.Equal(reflect.ValueOf(task), reflect.ValueOf(fixture.task))
	assertT.Same(&sema, fixture.sema)
	assertT.NotNil(fixture.runtimes)
}

func TestOneTaskSuccess(t *testing.T) {
	assertT := assert.New(t)

	taskOk := func() error {
		time.Sleep(SleepTime)
		return nil
	}
	fixture := createFixture(taskOk, &sema, &waitGroup)
	waitGroup.Add(1)
	runOneTask(fixture)

	assertT.Equal(1, len(fixture.runtimes))
	assertT.Equal(0, fixture.fails)
	assertT.GreaterOrEqual(fixture.runtimes[0], SleepTime)
}

func TestOneTaskFailure(t *testing.T) {
	assertT := assert.New(t)

	taskFail := func() error {
		time.Sleep(SleepTime)
		return errors.New("")
	}
	fixture := createFixture(taskFail, &sema, &waitGroup)
	waitGroup.Add(1)
	runOneTask(fixture)

	assertT.Equal(1, len(fixture.runtimes))
	assertT.Equal(1, fixture.fails)
	assertT.GreaterOrEqual(fixture.runtimes[0], SleepTime)
}

func TestCalcStats(t *testing.T) {
	assertT := assert.New(t)

	aFixture := taskFixture{runtimes: []time.Duration{9000000, 8000000, 7000000, 6000000, 5000000, 4000000, 3000000, 2000000, 1000000, 0}, fails: 3}
	stats := calcStats([]*taskFixture{&aFixture})

	assertT.Equal(1, len(stats))
	oneStat := stats[0]
	assertT.Equal(10, oneStat.Count)
	assertT.Equal(3, oneStat.Fails)
	assertT.Equal(0.0, oneStat.MinTime)
	assertT.Equal(4.5, oneStat.AvgTime)
	assertT.Equal(5.0, oneStat.MedTime)
	assertT.Equal(9.0, oneStat.MaxTime)
	assertT.Equal(3.0276503540974917, oneStat.StdDev)
	assertT.Equal([]float64{9.0, 8.0, 7.0, 6.0, 5.0, 4.0, 3.0, 2.0, 1.0, 0.0}, oneStat.Values)
}

func TestRunTest(t *testing.T) {
	assertT := assert.New(t)

	var callsCount atomic.Int32

	taskOk := func() error {
		time.Sleep(SleepTime)
		callsCount.Add(1)
		return nil
	}

	stats := RunTest([]TestTask{taskOk}, TotalTests, Parallel)

	assertT.Equal(TotalTests, int(callsCount.Load()))
	assertT.Equal(1, len(stats))
	oneStat := stats[0]
	assertT.Equal(TotalTests, oneStat.Count)
	assertT.GreaterOrEqual(oneStat.MinTime, float64(SleepTime)/msecFctr)
}

func TestIgnoreErr(t *testing.T) {
	assertT := assert.New(t)

	assertT.Equal(1, IgnoreErr(func() (int, error) { return 1, nil }, -1))
	assertT.Equal(-1, IgnoreErr(func() (int, error) { return 0, errTest }, -1))
}

func TestAssertNoErr(t *testing.T) {
	assertT := assert.New(t)

	assertT.Equal("Hello", AssertNoErr("Hello", nil))
	assertT.Panics(func() { AssertNoErr("Hello", errTest) })
}

func TestAssumeOnErr(t *testing.T) {
	assertT := assert.New(t)

	assertT.Equal(1, AssumeOnErr(func() (int, error) { return 1, nil }, -1))
	assertT.Equal(-1, AssumeOnErr(func() (int, error) { return 0, errTest }, -1))
}

var (
	stat1 = RunStats{Count: 100,
		AvgTime: 563.619080,
		MinTime: 523.003400,
		MaxTime: 763.448900,
		MedTime: 566.497100,
		StdDev:  563.61908,
	}
	stat2 = RunStats{Count: 200,
		AvgTime: 638.293246,
		MinTime: 617.243500,
		MaxTime: 694.757900,
		MedTime: 635.208100,
		StdDev:  12.563879,
	}
)

func TestCalcPvals(t *testing.T) {
	assertT := assert.New(t)

	probs, err := CalcPvals([]RunStats{stat1}, []RunStats{stat2})
	assertT.NoError(err)
	assertT.Equal(0.9691777120698255, probs[0])

	stat2.Count = 1
	_, err = CalcPvals([]RunStats{stat1}, []RunStats{stat2})
	assertT.ErrorContains(err, "invalid statistics data in test #0")
	assertT.ErrorContains(err, "sample is too small")

	stats1 := []RunStats{RunStats{}}
	stats2 := []RunStats{RunStats{}, RunStats{}}
	_, err = CalcPvals(stats1, stats2)
	assertT.ErrorContains(err, "different size of tasks")
}
