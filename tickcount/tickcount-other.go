//go:build !(386 || amd64 || ppc64)

package tickcount

func TickCount() uint64 {
	return 0
}

func TickCountOverhead() uint64 {
	return 0
}
