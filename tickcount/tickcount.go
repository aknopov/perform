//go:build 386 || amd64 || ppc64

package tickcount

import "math"

const (
	ovhdCnt = 10000
)

//go:noinline
func TickCountA() uint64

func TickCount() uint64 {
	return TickCountA()
}

// See https://community.intel.com/t5/Intel-ISA-Extensions/Measure-the-execution-time-using-RDTSC/td-p/1365538
func TickCountOverhead() uint64 {
	ovhd := uint64(math.MaxUint64)

	for i := 0; i < ovhdCnt; i++ {
		cnt0 := TickCount()
		delta := TickCount() - cnt0
		if delta < ovhd {
			ovhd = delta
		}
	}

	return ovhd
}
