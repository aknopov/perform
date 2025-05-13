package main

import (
	"github.com/ericlagergren/decimal"
)

var (
	ctx = decimal.Context128
)

// The function calculates sum of Flint-Hills series (https://arxiv.org/abs/1104.5100)
func calcSum(n int) *decimal.Big {

	one := decimal.New(1, 0)
	bN := decimal.New(int64(n), 0)
	bI := new(decimal.Big)
	term := new(decimal.Big)
	sum := new(decimal.Big)
	for bI.SetUint64(1); bI.Cmp(bN) <= 0; bI.Add(bI, one) {

		ctx.Sin(term, bI)         // sin(n)
		ctx.Mul(term, term, bI)   // n*sin(n)
		ctx.Mul(term, term, term) // (n*sin(n))^2
		ctx.Mul(term, term, bI)   // n*(n*sin(n))^2
		ctx.Quo(term, one, term)  // 1/(n*(n*sin(n))^2)

		ctx.Add(sum, sum, term)
	}

	return sum
}
