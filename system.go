package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var cmds = map[string]string{
	"blk":  "lsblk | awk '{print $1}'",
	"up":   "echo Uptime: `uptime | sed 's/.*up \\([^,]*\\), .*/\\1/'`",
	"temp": "cat /sys/class/thermal/thermal_zone0/temp",
	"ip":   "hostname -I | awk '{printf \"IP %s\", $1}'",
	"cpu":  "uptime | awk '{printf \"CPU Load: %.2f\", $(NF-2)}'",
	"men":  "free -m | awk 'NR==2{printf \"Mem: %s/%sMB\", $3,$2}'",
	"disk": "df -h | awk '$NF==\"/\"{printf \"Disk: %d/%dGB %s\", $3,$2,$5}'",
}

// CheckOutput runs a shell command and returns its trimmed stdout.
func CheckOutput(cmd string) string {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CheckCall runs a shell command and returns any error.
func CheckCall(cmd string) error {
	return exec.Command("sh", "-c", cmd).Run()
}

// GetInfo runs the named system command and returns its output.
func GetInfo(key string) string {
	cmd, ok := cmds[key]
	if !ok {
		return ""
	}
	return CheckOutput(cmd)
}

// GetCPUTemp returns a formatted CPU temperature string.
func GetCPUTemp(fTemp bool) string {
	raw := GetInfo("temp")
	millideg, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "CPU Temp: N/A"
	}
	t := millideg / 1000.0
	if fTemp {
		return fmt.Sprintf("CPU Temp: %.0f\u00b0F", t*1.8+32)
	}
	return fmt.Sprintf("CPU Temp: %.1f\u00b0C", t)
}

// ReadTemp reads the CPU temperature in degrees Celsius from sysfs.
func ReadTemp() float64 {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0
	}
	millideg, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0
	}
	return millideg / 1000.0
}

// --- Disk info with caching ---

var diskInfoCache struct {
	mu   sync.Mutex
	keys []string
	vals []string
	when time.Time
}

// GetDiskInfo returns disk labels and usage percentages, cached for 30s.
func GetDiskInfo(cfg *Config) (keys []string, vals []string) {
	diskInfoCache.mu.Lock()
	defer diskInfoCache.mu.Unlock()

	if time.Since(diskInfoCache.when) > 30*time.Second {
		k := []string{"root"}
		v := []string{CheckOutput(`df -h | awk '$NF=="/"{printf "%s", $5}'`)}

		for _, disk := range cfg.GetDisks() {
			cmd := fmt.Sprintf(`df -Bg | awk '$1=="/dev/%s" {printf "%%s", $5}'`, disk)
			k = append(k, disk)
			v = append(v, CheckOutput(cmd))
		}

		diskInfoCache.keys = k
		diskInfoCache.vals = v
		diskInfoCache.when = time.Now()
	}

	return diskInfoCache.keys, diskInfoCache.vals
}
