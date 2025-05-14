package main

import (
	"fmt"
	"testing"

	"github.com/ericlagergren/decimal"
)

// UC

func TestSum(t *testing.T) {
	fmt.Printf("%s\n", calcSum(10000))
	fmt.Printf("%s\n", calcSum(20000))
	fmt.Printf("%s\n", calcSum(30000))
}

var val *decimal.Big

func BenchmarkLim10(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		calcSum(20000)
	}
}

func BenchmarkLim12(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val = calcSum(11)
	}
	fmt.Printf("Sum = %s\n", val)
}

func BenchmarkLim16(b *testing.B) {
	for i := 0; i < b.N; i++ {
		val = calcSum(16)
	}
	fmt.Printf("%s\n", val)
}
