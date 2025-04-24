package main

import (
	"testing"
)

const (
	Password = "A secret phrase that should be quite long"
)

func BenchmarkHashing16(b *testing.B) {
	request := HashRequest{Password, 16}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash(&request)
	}
}

func BenchmarkHashing32(b *testing.B) {
	request := HashRequest{Password, 32}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash(&request)
	}
}

func BenchmarkHashing64(b *testing.B) {
	request := HashRequest{Password, 64}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash(&request)
	}
}
