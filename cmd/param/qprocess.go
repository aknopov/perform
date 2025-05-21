package param

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// In leu of interface(s) in "gopsutil" library - only necessary methods
type IQProcess interface {
	GetPID() int32
	MemoryInfo() (*process.MemoryInfoStat, error)
	NetIOCounters(pernic bool) ([]net.IOCountersStat, error) // disappeared in v4
	NumThreads() (int32, error)
	Times() (*cpu.TimesStat, error)
	Percent(time.Duration) (float64, error)
}

// Wrapper for process.Process for mocking in public
type QProcess struct {
	*process.Process
}

var (
	NO_NET_IO = []net.IOCountersStat{}
)

// Creates new instance to avoid "struct literal uses unkeyed fields"
func NewQProcess(p *process.Process) *QProcess {
	return &QProcess{p}
}

func (p *QProcess) GetPID() int32 {
	return p.Pid
}

func (p *QProcess) NetIOCounters(pernic bool) ([]net.IOCountersStat, error) { // UC
	return NO_NET_IO, nil
}
