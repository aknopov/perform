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

func check3(t *testing.T, test func(alt LocationHypothesis) (*TTestResult, error), n1, n2 int, tval, dof float64, pless, pdiff, pgreater float64) {
	t.Helper()
	want := &TTestResult{N1: n1, N2: n2, T: tval, DoF: dof}

	want.AltHypothesis = LocationLess
	want.P = pless
	got, _ := test(want.AltHypothesis)
	check(t, want, got)

	want.AltHypothesis = LocationDiffers
	want.P = pdiff
	got, _ = test(want.AltHypothesis)
	check(t, want, got)

	want.AltHypothesis = LocationGreater
	want.P = pgreater
	got, _ = test(want.AltHypothesis)
	check(t, want, got)
}

func TestTTest(t *testing.T) {

	check3(t, func(alt LocationHypothesis) (*TTestResult, error) {
		return TwoSampleTTest(s1, s1, alt)
	}, 4, 4, 0, 6,
		0.5, 1, 0.5)
	check3(t, func(alt LocationHypothesis) (*TTestResult, error) {
		return TwoSampleWelchTTest(s1, s1, alt)
	}, 4, 4, 0, 6,
		0.5, 1, 0.5)

	check3(t, func(alt LocationHypothesis) (*TTestResult, error) {
		return TwoSampleTTest(s1, s2, alt)
	}, 4, 4, -3.9703446152237674, 6,
		0.0036820296121056195, 0.0073640592242113214, 0.9963179703878944)
	check3(t, func(alt LocationHypothesis) (*TTestResult, error) {
		return TwoSampleWelchTTest(s1, s2, alt)
	}, 4, 4, -3.9703446152237674, 5.584615384615385,
		0.004256431565689112, 0.0085128631313781695, 0.9957435684343109)

	check3(t, func(alt LocationHypothesis) (*TTestResult, error) {
		return PairedTTest(vals1, vals2, alt)
	}, 4, 4, -17, 3,
		0.0002216717691559955, 0.00044334353831207749, 0.999778328230844)
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
}

func BenchmarkTwoSampleTTest(b *testing.B) {
	b.ResetTimer()

	for range b.N {
		TwoSampleTTest(s1, s2, LocationDiffers) //nolint:errcheck
	}
}
