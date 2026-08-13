package handler

import (
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

// systemInfo is the live resource snapshot returned by GET /api/admin/system.
// Mirrors the idrouter System Monitor page: host + process metrics for the
// admin to gauge device performance and available storage capacity.
type systemInfo struct {
	CPUPct     float64   `json:"cpu_pct"`
	CPUPerCore []float64 `json:"cpu_per_core"`
	CPUCount   int       `json:"cpu_count"`

	MemTotalMB     uint64  `json:"mem_total_mb"`
	MemUsedMB      uint64  `json:"mem_used_mb"`
	MemAvailableMB uint64  `json:"mem_available_mb"`
	MemPct         float64 `json:"mem_pct"`

	DiskTotalGB float64 `json:"disk_total_gb"`
	DiskUsedGB  float64 `json:"disk_used_gb"`
	DiskFreeGB  float64 `json:"disk_free_gb"`
	DiskPct     float64 `json:"disk_pct"`

	Host    string `json:"host"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	PID     int    `json:"pid"`
	UptimeS int64  `json:"uptime_s"`

	Goroutines  int     `json:"goroutines"`
	HeapAllocMB float64 `json:"heap_alloc_mb"`

	ProcCPUPct  float64 `json:"proc_cpu_pct"`
	ProcRSSMB   float64 `json:"proc_rss_mb"`
	ProcThreads int32   `json:"proc_threads"`
	ProcFDs     int32   `json:"proc_fds"`
}

// AdminSystemInfo returns a point-in-time snapshot of host and process
// resource usage for the admin System monitor page.
func AdminSystemInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		info := collectSystemInfo()
		Success(c, info)
	}
}

func collectSystemInfo() systemInfo {
	var s systemInfo

	// CPU
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		s.CPUPct = pcts[0]
	}
	if pcts, err := cpu.Percent(0, true); err == nil {
		s.CPUPerCore = pcts
	}
	if n, err := cpu.Counts(true); err == nil {
		s.CPUCount = n
	}

	// Memory
	if vm, err := mem.VirtualMemory(); err == nil {
		s.MemTotalMB = vm.Total / (1024 * 1024)
		s.MemUsedMB = vm.Used / (1024 * 1024)
		s.MemAvailableMB = vm.Available / (1024 * 1024)
		s.MemPct = vm.UsedPercent
	}

	// Disk (root partition) — reflects the capacity available on the device
	// where the server runs.
	if usage, err := disk.Usage("/"); err == nil {
		s.DiskTotalGB = float64(usage.Total) / (1024 * 1024 * 1024)
		s.DiskUsedGB = float64(usage.Used) / (1024 * 1024 * 1024)
		s.DiskFreeGB = float64(usage.Free) / (1024 * 1024 * 1024)
		s.DiskPct = usage.UsedPercent
	}

	// Host info
	if hi, err := host.Info(); err == nil {
		s.Host = hi.Hostname
		s.OS = hi.Platform
		if hi.PlatformVersion != "" {
			s.OS += " " + hi.PlatformVersion
		}
		s.Arch = hi.KernelArch
	}
	s.PID = os.Getpid()
	s.UptimeS = int64(time.Since(startTime).Seconds())

	// Go runtime
	s.Goroutines = runtime.NumGoroutine()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.HeapAllocMB = float64(ms.HeapAlloc) / (1024 * 1024)

	// Process-level metrics
	pid := int32(os.Getpid())
	if p, err := process.NewProcess(pid); err == nil {
		if pct, err := p.Percent(0); err == nil {
			s.ProcCPUPct = pct
		}
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			s.ProcRSSMB = float64(mi.RSS) / (1024 * 1024)
		}
		if t, err := p.NumThreads(); err == nil {
			s.ProcThreads = t
		}
		if fds, err := p.NumFDs(); err == nil {
			s.ProcFDs = fds
		}
	}

	return s
}

// startTime records when the server process started, used for uptime display.
var startTime = time.Now()
