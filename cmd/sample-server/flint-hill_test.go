package main

import (
	"testing"

	"github.com/ericlagergren/decimal"
	"github.com/stretchr/testify/assert"
)

func TestSum(t *testing.T) {
	lowEst := (&decimal.Big{}).SetFloat64(30.3145)

	assert.True(t, lowEst.Cmp(calcSum(10000)) < 0)
	assert.True(t, lowEst.Cmp(calcSum(20000)) < 0)
	assert.True(t, lowEst.Cmp(calcSum(30000)) < 0)
}

func BenchmarkLim20000(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		calcSum(20000)
	}
}
