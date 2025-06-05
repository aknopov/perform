package perform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	vals1 = []float64{2, 1, 3, 4}
	vals2 = []float64{6, 5, 7, 9}
	s1    = tTestSample{weight: float64(len(vals1)), mean: mean(vals1), variance: variance(vals1)}
	s2    = tTestSample{weight: float64(len(vals2)), mean: mean(vals2), variance: variance(vals2)}
)

// aeq returns true if expect and got are equal to 8 significant
// figures (1 part in 100 million).
func aeq(expect, got float64) bool {
	if expect < 0 && got < 0 {
		expect, got = -expect, -got
	}
	return expect*0.99999999 <= got && got*0.99999999 <= expect
}

// ae checks that function returns error with specified message
func ae(t *testing.T, f func() (*TTestResult, error), msg string) {
	_, err := f()
	assert.ErrorContains(t, err, msg)
}

func check(t *testing.T, want, got *TTestResult) {
	t.Helper()
	if want.N1 != got.N1 || want.N2 != got.N2 ||
		!aeq(want.T, got.T) || !aeq(want.DoF, got.DoF) ||
		want.AltHypothesis != got.AltHypothesis ||
		!aeq(want.P, got.P) {
		t.Errorf("want %+v, got %+v", want, got)
	}
}

type testData[X any] struct {
	fun       func(x1, x2 X, alt LocationHypothesis) (*TTestResult, error)
	x1, x2    X
	n1, n2    int
	tval, dof float64
	alt       LocationHypothesis
	p         float64
}

func TestTTest(t *testing.T) {
	unpairedData := []testData[tTestSample]{
		{TwoSampleTTest, s1, s1, 4, 4, 0, 6, LocationLess, 0.5},
		{TwoSampleTTest, s1, s1, 4, 4, 0, 6, LocationDiffers, 1.0},
		{TwoSampleTTest, s1, s1, 4, 4, 0, 6, LocationGreater, 0.5},
		{TwoSampleTTest, s1, s2, 4, 4, -3.9703446152237674, 6, LocationLess, 0.0036820296121056195},
		{TwoSampleTTest, s1, s2, 4, 4, -3.9703446152237674, 6, LocationDiffers, 0.0073640592242113214},
		{TwoSampleTTest, s1, s2, 4, 4, -3.9703446152237674, 6, LocationGreater, 0.9963179703878944},
		{TwoSampleWelchTTest, s1, s1, 4, 4, 0, 6, LocationLess, 0.5},
		{TwoSampleWelchTTest, s1, s1, 4, 4, 0, 6, LocationDiffers, 1.0},
		{TwoSampleWelchTTest, s1, s1, 4, 4, 0, 6, LocationGreater, 0.5},
		{TwoSampleWelchTTest, s1, s2, 4, 4, -3.9703446152237674, 5.584615384615385, LocationLess, 0.004256431565689112},
		{TwoSampleWelchTTest, s1, s2, 4, 4, -3.9703446152237674, 5.584615384615385, LocationDiffers, 0.0085128631313781695},
		{TwoSampleWelchTTest, s1, s2, 4, 4, -3.9703446152237674, 5.584615384615385, LocationGreater, 0.9957435684343109},
	}
	for _, d := range unpairedData {
		want := &TTestResult{N1: d.n1, N2: d.n2, T: d.tval, DoF: d.dof, AltHypothesis: d.alt, P: d.p}
		got, _ := d.fun(d.x1, d.x2, d.alt)
		check(t, want, got)
	}

	pairedData := []testData[[]float64]{
		{PairedTTest, vals1, vals2, 4, 4, -17, 3, LocationLess, 0.0002216717691559955},
		{PairedTTest, vals1, vals2, 4, 4, -17, 3, LocationDiffers, 0.00044334353831207749},
		{PairedTTest, vals1, vals2, 4, 4, -17, 3, LocationGreater, 0.999778328230844},
	}
	for _, d := range pairedData {
		want := &TTestResult{N1: d.n1, N2: d.n2, T: d.tval, DoF: d.dof, AltHypothesis: d.alt, P: d.p}
		got, _ := d.fun(d.x1, d.x2, d.alt)
		check(t, want, got)
	}
}

func TestFailures(t *testing.T) {
	ts0 := tTestSample{weight: 2, variance: 0.5}
	ts1 := tTestSample{weight: 1, variance: 0.5}
	ts2 := tTestSample{weight: 2, variance: 0.0}

	ae(t, func() (*TTestResult, error) { return TwoSampleTTest(ts0, ts1, LocationDiffers) }, "sample is too small")
	ae(t, func() (*TTestResult, error) { return TwoSampleTTest(ts1, ts0, LocationDiffers) }, "sample is too small")
	ae(t, func() (*TTestResult, error) { return TwoSampleTTest(ts2, ts2, LocationDiffers) }, "sample has zero variance")

	ae(t, func() (*TTestResult, error) { return TwoSampleWelchTTest(ts0, ts1, LocationDiffers) }, "sample is too small")
	ae(t, func() (*TTestResult, error) { return TwoSampleWelchTTest(ts1, ts0, LocationDiffers) }, "sample is too small")
	ae(t, func() (*TTestResult, error) { return TwoSampleWelchTTest(ts2, ts2, LocationDiffers) }, "sample has zero variance")

	vals3 := []float64{1.0}
	ae(t, func() (*TTestResult, error) { return PairedTTest(vals1, vals3, LocationDiffers) }, "samples have different lengths")
	ae(t, func() (*TTestResult, error) { return PairedTTest(vals3, vals1, LocationDiffers) }, "samples have different lengths")
	ae(t, func() (*TTestResult, error) { return PairedTTest(vals3, vals3, LocationDiffers) }, "sample is too small")
	ae(t, func() (*TTestResult, error) { return PairedTTest(vals1, vals1, LocationDiffers) }, "sample has zero variance")
}

func BenchmarkTwoSampleTTest(b *testing.B) {
	b.ResetTimer()

	for range b.N {
		TwoSampleTTest(s1, s2, LocationDiffers) //nolint:errcheck
	}
}
