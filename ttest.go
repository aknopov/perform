package perform

// Taken from "golang.org/x/perf/internal/stats":
// Copyright 2009 The Go Authors.

import (
	"errors"
	"math"
)

// A LocationHypothesis specifies the alternative hypothesis of a
// location test such as a t-test. The default (zero) value is to test
//
//	against the alternative hypothesis that they differ.
type LocationHypothesis int

const (
	// LocationLess specifies the alternative hypothesis that the
	// location of the first sample is less than the second. This
	// is a one-tailed test.
	LocationLess LocationHypothesis = -1

	// LocationDiffers specifies the alternative hypothesis that
	// the locations of the two samples are not equal. This is a
	// two-tailed test.
	LocationDiffers LocationHypothesis = 0

	// LocationGreater specifies the alternative hypothesis that
	// the location of the first sample is greater than the
	// second. This is a one-tailed test.
	LocationGreater LocationHypothesis = 1
)

// A TTestResult is the result of a t-test.
type TTestResult struct {
	// N1 and N2 are the sizes of the input samples. For a
	// one-sample t-test, N2 is 0.
	N1, N2 int

	// T is the value of the t-statistic for this t-test.
	T float64

	// DoF is the degrees of freedom for this t-test.
	DoF float64

	// AltHypothesis specifies the alternative hypothesis tested
	// by this test against the null hypothesis that there is no
	// difference in the means of the samples.
	AltHypothesis LocationHypothesis

	// P is p-value for this t-test for the given null hypothesis.
	P float64
}

// A tDist is a Student's t-distribution with V degrees of freedom.
type tDist struct {
	v float64
}

// A TTestSample is a sample that can be used for a one or two sample t-test.
type tTestSample struct {
	weight   float64
	mean     float64
	variance float64
}

var (
	ErrSampleSize        = errors.New("sample is too small")
	ErrZeroVariance      = errors.New("sample has zero variance")
	ErrMismatchedSamples = errors.New("samples have different lengths")
)

func (t tDist) cdf(x float64) float64 {
	switch {
	case x == 0:
		return 0.5
	case x > 0:
		return 1 - 0.5*mathBetaInc(t.v/(t.v+x*x), t.v/2, 0.5)
	case x < 0:
		return 1 - t.cdf(-x)
	default:
		return math.NaN()
	}
}

func timeStat2Tstat(rs RunStats) tTestSample {
	return tTestSample{weight: float64(rs.Count), mean: float64(rs.AvgTime), variance: float64(rs.StdDev) * float64(rs.StdDev)}
}

func newTTestResult(n1, n2 int, t, dof float64, alt LocationHypothesis) *TTestResult {
	dist := tDist{dof}
	var p float64
	switch alt {
	case LocationDiffers:
		p = 2 * (1 - dist.cdf(math.Abs(t)))
	case LocationLess:
		p = dist.cdf(t)
	case LocationGreater:
		p = 1 - dist.cdf(t)
	}
	return &TTestResult{N1: n1, N2: n2, T: t, DoF: dof, AltHypothesis: alt, P: p}
}

// TwoSampleTTest performs a two-sample (unpaired) Student's t-test on
// samples x1 and x2. This is a test of the null hypothesis that x1
// and x2 are drawn from populations with equal means. It assumes x1
// and x2 are independent samples, that the distributions have equal
// variance, and that the populations are normally distributed.
func TwoSampleTTest(x1, x2 tTestSample, alt LocationHypothesis) (*TTestResult, error) {
	n1, n2 := x1.weight, x2.weight
	if n1 <= 1 || n2 <= 1 {
		return nil, ErrSampleSize
	}
	v1, v2 := x1.variance, x2.variance
	if v1 == 0 && v2 == 0 {
		return nil, ErrZeroVariance
	}

	dof := n1 + n2 - 2
	v12 := ((n1-1)*v1 + (n2-1)*v2) / dof
	t := (x1.mean - x2.mean) / math.Sqrt(v12*(1/n1+1/n2))
	return newTTestResult(int(n1), int(n2), t, dof, alt), nil
}

// TwoSampleWelchTTest performs a two-sample (unpaired) Welch's t-test
// on samples x1 and x2. This is like TwoSampleTTest, but does not
// assume the distributions have equal variance.
func TwoSampleWelchTTest(x1, x2 tTestSample, alt LocationHypothesis) (*TTestResult, error) {
	n1, n2 := x1.weight, x2.weight
	if n1 <= 1 || n2 <= 1 {
		return nil, ErrSampleSize
	}
	v1, v2 := x1.variance, x2.variance
	if v1 == 0 && v2 == 0 {
		return nil, ErrZeroVariance
	}

	dof := (v1/n1 + v2/n2) * (v1/n1 + v2/n2) /
		(v1/n1*(v1/n1)/(n1-1) + v2/n2*(v2/n2)/(n2-1))
	s := math.Sqrt(v1/n1 + v2/n2)
	t := (x1.mean - x2.mean) / s
	return newTTestResult(int(n1), int(n2), t, dof, alt), nil
}

// PairedTTest performs a two-sample paired t-test on samples x1 and x2.
// If x1 and x2 are identical, this returns nil.
func PairedTTest(x1, x2 []float64, alt LocationHypothesis) (*TTestResult, error) {
	if len(x1) != len(x2) {
		return nil, ErrMismatchedSamples
	}
	if len(x1) <= 1 {
		return nil, ErrSampleSize
	}

	dof := float64(len(x1) - 1)

	diff := make([]float64, len(x1))
	for i := range x1 {
		diff[i] = x1[i] - x2[i]
	}
	sd := math.Sqrt(variance(diff))
	if sd == 0 {
		return nil, ErrZeroVariance
	}
	t := mean(diff) * math.Sqrt(float64(len(x1))) / sd
	return newTTestResult(len(x1), len(x2), t, dof, alt), nil
}

func lgamma(x float64) float64 {
	y, _ := math.Lgamma(x)
	return y
}

// mathBetaInc returns the value of the regularized incomplete beta
// function Iₓ(a, b).
//
// This is not to be confused with the "incomplete beta function",
// which can be computed as BetaInc(x, a, b)*Beta(a, b).
//
// If x < 0 or x > 1, returns NaN.
func mathBetaInc(x, a, b float64) float64 {
	// Based on Numerical Recipes in C, section 6.4. This uses the
	// continued fraction definition of I:
	//
	//  (xᵃ*(1-x)ᵇ)/(a*B(a,b)) * (1/(1+(d₁/(1+(d₂/(1+...))))))
	//
	// where B(a,b) is the beta function and
	//
	//  d_{2m+1} = -(a+m)(a+b+m)x/((a+2m)(a+2m+1))
	//  d_{2m}   = m(b-m)x/((a+2m-1)(a+2m))
	if x < 0 || x > 1 {
		return math.NaN()
	}
	bt := 0.0
	if 0 < x && x < 1 {
		// Compute the coefficient before the continued
		// fraction.
		bt = math.Exp(lgamma(a+b) - lgamma(a) - lgamma(b) +
			a*math.Log(x) + b*math.Log(1-x))
	}
	if x < (a+1)/(a+b+2) {
		// Compute continued fraction directly.
		return bt * betacf(x, a, b) / a
	} else {
		// Compute continued fraction after symmetry transform.
		return 1 - bt*betacf(1-x, b, a)/b
	}
}

// betacf is the continued fraction component of the regularized
// incomplete beta function Iₓ(a, b).
func betacf(x, a, b float64) float64 {
	const maxIterations = 200
	const epsilon = 3e-14

	raiseZero := func(z float64) float64 {
		if math.Abs(z) < math.SmallestNonzeroFloat64 {
			return math.SmallestNonzeroFloat64
		}
		return z
	}

	c := 1.0
	d := 1 / raiseZero(1-(a+b)*x/(a+1))
	h := d
	for m := 1; m <= maxIterations; m++ {
		mf := float64(m)

		// Even step of the recurrence.
		numer := mf * (b - mf) * x / ((a + 2*mf - 1) * (a + 2*mf))
		d = 1 / raiseZero(1+numer*d)
		c = raiseZero(1 + numer/c)
		h *= d * c

		// Odd step of the recurrence.
		numer = -(a + mf) * (a + b + mf) * x / ((a + 2*mf) * (a + 2*mf + 1))
		d = 1 / raiseZero(1+numer*d)
		c = raiseZero(1 + numer/c)
		hfac := d * c
		h *= hfac

		if math.Abs(hfac-1) < epsilon {
			return h
		}
	}
	panic("betainc: a or b too big; failed to converge")
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	m := 0.0
	for i, x := range xs {
		m += (x - m) / float64(i+1)
	}
	return m
}

func variance(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	} else if len(xs) <= 1 {
		return 0
	}

	// Based on Wikipedia's presentation of Welford 1962
	// (http://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Online_algorithm).
	// This is more numerically stable than the standard two-pass
	// formula and not prone to massive cancellation.
	mean, M2 := 0.0, 0.0
	for n, x := range xs {
		delta := x - mean
		mean += delta / float64(n+1)
		M2 += delta * (x - mean)
	}
	return M2 / float64(len(xs)-1)
}
