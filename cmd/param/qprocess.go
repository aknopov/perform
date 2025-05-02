package param

import (
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

// In leu of interface(s) in "gopsutil" library - only necessary methods
type IQProcess interface {
	GetPID() int32
	MemoryInfo() (*process.MemoryInfoStat, error)
	NetIOCounters(pernic bool) ([]net.IOCountersStat, error)
	NumThreads() (int32, error)
	Times() (*cpu.TimesStat, error)
	Percent(time.Duration) (float64, error)
}

// Wrapper for process.Process for mocking in public
type QProcess struct {
	*process.Process
}

// Creates new instance to avoid "struct literal uses unkeyed fields"
func NewQProcess(p *process.Process) *QProcess {
	return &QProcess{p}
}

func (p *QProcess) GetPID() int32 {
	return p.Pid
}
