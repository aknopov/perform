//go:build 386 || amd64 || ppc64

package tickcount

//go:noinline
func TickCountA() uint64

func TickCount() uint64 {
	return TickCountA()
}
