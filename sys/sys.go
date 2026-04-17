package sys

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anatol/smart.go"
)

// GetUptime returns a formatted uptime string.
func GetUptime() string {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return "Uptime: N/A"
	}
	s := si.Uptime
	d := s / 86400
	h := (s % 86400) / 3600
	m := (s % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("Uptime: %dd %dh %dm", d, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("Uptime: %dh %dm", h, m)
	}
	return fmt.Sprintf("Uptime: %dm", m)
}

// GetIP returns the first non-loopback IPv4 address.
func GetIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "IP N/A"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					return fmt.Sprintf("IP %s", ip4)
				}
			}
		}
	}
	return "IP N/A"
}

// GetCPULoad returns a formatted 1-minute CPU load average string.
func GetCPULoad() string {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return "CPU Load: N/A"
	}
	load := float64(si.Loads[0]) / 65536.0
	return fmt.Sprintf("CPU Load: %.2f", load)
}

// GetMemory returns a formatted used/total memory string in MB.
func GetMemory() string {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return "Mem: N/A"
	}
	unit := uint64(si.Unit)
	total := si.Totalram * unit / 1024 / 1024
	used := (si.Totalram - si.Freeram) * unit / 1024 / 1024
	return fmt.Sprintf("Mem: %d/%dMB", used, total)
}

// GetCPUTemp returns a formatted CPU temperature string.
func GetCPUTemp(fTemp bool) string {
	t := ReadTemp()
	if t == 0 {
		return "CPU Temp: N/A"
	}
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

// DriveTemps returns the temperature in °C for each readable drive.
func DriveTemps(disks []string) map[string]float64 {
	temps := make(map[string]float64)
	for _, disk := range disks {
		dev, err := smart.Open("/dev/" + disk)
		if err != nil {
			continue
		}
		attrs, err := dev.ReadGenericAttributes()
		dev.Close()
		if err != nil {
			continue
		}
		if attrs.Temperature > 0 {
			temps[disk] = float64(attrs.Temperature)
		}
	}
	return temps
}

// DriveTemp returns the highest temperature across all SATA drives in °C.
// Returns 0 if no drives are present or temperatures cannot be read.
func DriveTemp(disks []string) float64 {
	var maxTemp float64
	for _, t := range DriveTemps(disks) {
		if t > maxTemp {
			maxTemp = t
		}
	}
	return maxTemp
}

// --- Disk info and temperature with caching ---

var diskTempCache struct {
	mu   sync.Mutex
	keys []string
	vals []string
	when time.Time
}

// GetDiskTemps returns SATA drive labels and formatted temperatures, cached for 30s.
func GetDiskTemps(disks []string) (keys []string, vals []string) {
	diskTempCache.mu.Lock()
	defer diskTempCache.mu.Unlock()

	if time.Since(diskTempCache.when) > 30*time.Second {
		temps := DriveTemps(disks)
		var k, v []string
		for _, disk := range disks {
			k = append(k, disk)
			if t, ok := temps[disk]; ok {
				v = append(v, fmt.Sprintf("%.0f\u00b0C", t))
			} else {
				v = append(v, "N/A")
			}
		}
		diskTempCache.keys = k
		diskTempCache.vals = v
		diskTempCache.when = time.Now()
	}

	return diskTempCache.keys, diskTempCache.vals
}

var diskInfoCache struct {
	mu   sync.Mutex
	keys []string
	vals []string
	when time.Time
}

// DiskUsagePct returns the used-space percentage for the filesystem at path.
func DiskUsagePct(path string) string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil || stat.Blocks == 0 {
		return "N/A"
	}
	pct := (stat.Blocks - stat.Bavail) * 100 / stat.Blocks
	return fmt.Sprintf("%d%%", pct)
}

// MountpointOf returns the mountpoint for the given block device by parsing /proc/mounts.
func MountpointOf(dev string) string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == dev {
			return fields[1]
		}
	}
	return ""
}

// GetDiskUsage returns disk labels and usage percentages, cached for 30s.
func GetDiskUsage(disks []string) (keys []string, vals []string) {
	diskInfoCache.mu.Lock()
	defer diskInfoCache.mu.Unlock()

	if time.Since(diskInfoCache.when) > 30*time.Second {
		k := []string{"root"}
		v := []string{DiskUsagePct("/")}

		for _, disk := range disks {
			mp := MountpointOf("/dev/" + disk)
			var pct string
			if mp == "" {
				pct = "N/A"
			} else {
				pct = DiskUsagePct(mp)
			}
			k = append(k, disk)
			v = append(v, pct)
		}

		diskInfoCache.keys = k
		diskInfoCache.vals = v
		diskInfoCache.when = time.Now()
	}

	return diskInfoCache.keys, diskInfoCache.vals
}
