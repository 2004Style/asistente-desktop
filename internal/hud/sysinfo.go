package hud

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SysInfo struct {
	mu      sync.RWMutex
	CPU     float64
	RAM     float64
	GPU     float64
	lastCPU struct {
		idle  uint64
		total uint64
	}
}

func NewSysInfo() *SysInfo {
	return &SysInfo{}
}

func (s *SysInfo) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *SysInfo) Get() (cpu, ram, gpu float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CPU, s.RAM, s.GPU
}

func (s *SysInfo) loop(ctx context.Context) {
	// Inicializar CPU, RAM y GPU al inicio
	s.updateCPU()
	s.updateRAM()
	s.updateGPU()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	gpuTicker := time.NewTicker(3 * time.Second)
	defer gpuTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateCPU()
			s.updateRAM()
		case <-gpuTicker.C:
			s.updateGPU()
		}
	}
}

func (s *SysInfo) updateRAM() {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer file.Close()

	var total, available uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				total, _ = strconv.ParseUint(parts[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				available, _ = strconv.ParseUint(parts[1], 10, 64)
			}
		}
	}

	if total > 0 {
		s.mu.Lock()
		if available > 0 {
			s.RAM = 100.0 * (1.0 - float64(available)/float64(total))
		} else {
			s.RAM = 0.0
		}
		s.mu.Unlock()
	}
}

func (s *SysInfo) updateCPU() {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return
	}

	parts := strings.Fields(line)
	if len(parts) < 5 {
		return
	}

	var user, nice, system, idle, iowait, irq, softirq, steal uint64
	user, _ = strconv.ParseUint(parts[1], 10, 64)
	nice, _ = strconv.ParseUint(parts[2], 10, 64)
	system, _ = strconv.ParseUint(parts[3], 10, 64)
	idle, _ = strconv.ParseUint(parts[4], 10, 64)
	if len(parts) > 5 {
		iowait, _ = strconv.ParseUint(parts[5], 10, 64)
	}
	if len(parts) > 6 {
		irq, _ = strconv.ParseUint(parts[6], 10, 64)
	}
	if len(parts) > 7 {
		softirq, _ = strconv.ParseUint(parts[7], 10, 64)
	}
	if len(parts) > 8 {
		steal, _ = strconv.ParseUint(parts[8], 10, 64)
	}

	idleTicks := idle + iowait
	totalTicks := user + nice + system + idleTicks + irq + softirq + steal

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastCPU.total > 0 {
		diffIdle := idleTicks - s.lastCPU.idle
		diffTotal := totalTicks - s.lastCPU.total
		if diffTotal > 0 {
			s.CPU = 100.0 * (1.0 - float64(diffIdle)/float64(diffTotal))
		}
	}
	s.lastCPU.idle = idleTicks
	s.lastCPU.total = totalTicks
}

func (s *SysInfo) updateGPU() {
	// 1. Intentar leer de /sys/class/drm/card0/device/gpu_busy_percent (AMD/Intel)
	if data, err := os.ReadFile("/sys/class/drm/card0/device/gpu_busy_percent"); err == nil {
		val := strings.TrimSpace(string(data))
		if pct, err := strconv.ParseFloat(val, 64); err == nil {
			s.mu.Lock()
			s.GPU = pct
			s.mu.Unlock()
			return
		}
	}

	// 2. Intentar leer de /sys/class/drm/card1/device/gpu_busy_percent
	if data, err := os.ReadFile("/sys/class/drm/card1/device/gpu_busy_percent"); err == nil {
		val := strings.TrimSpace(string(data))
		if pct, err := strconv.ParseFloat(val, 64); err == nil {
			s.mu.Lock()
			s.GPU = pct
			s.mu.Unlock()
			return
		}
	}

	// 3. Fallback a nvidia-smi para NVIDIA
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		val := strings.TrimSpace(out.String())
		if pct, err := strconv.ParseFloat(val, 64); err == nil {
			s.mu.Lock()
			s.GPU = pct
			s.mu.Unlock()
			return
		}
	}

	// Si no se encuentra GPU o no reporta uso
	s.mu.Lock()
	s.GPU = 0.0
	s.mu.Unlock()
}
