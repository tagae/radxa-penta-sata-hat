package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type Config struct {
	Fan    FanConfig
	Key    KeyConfig
	Time   TimeConfig
	Slider SliderConfig
	OLED   OLEDConfig

	disks  []string
	diskMu sync.Mutex

	idx int64 // atomic page counter
	run int64 // atomic fan enable flag (1=on, 0=off)
}

type FanConfig struct {
	Lv0, Lv1, Lv2, Lv3 float64
}

type KeyConfig struct {
	Click, Twice, Press string
}

type TimeConfig struct {
	Twice, Press float64
}

type SliderConfig struct {
	Auto bool
	Time float64
}

type OLEDConfig struct {
	Rotate bool
	FTemp  bool
}

// LoadConfig reads the INI-style configuration file. If reading fails,
// sensible defaults are used.
func LoadConfig(path string) *Config {
	cfg := &Config{}
	atomic.StoreInt64(&cfg.idx, -1)
	atomic.StoreInt64(&cfg.run, 1)

	// Defaults matching the original Python implementation.
	cfg.Fan = FanConfig{Lv0: 35, Lv1: 40, Lv2: 45, Lv3: 50}
	cfg.Key = KeyConfig{Click: "slider", Twice: "switch", Press: "none"}
	cfg.Time = TimeConfig{Twice: 0.7, Press: 1.8}
	cfg.Slider = SliderConfig{Auto: true, Time: 10}
	cfg.OLED = OLEDConfig{Rotate: false, FTemp: false}

	sections, err := parseINI(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v, using defaults\n", err)
		return cfg
	}

	// Fan thresholds (°C).
	if s := sections["fan"]; s != nil {
		cfg.Fan.Lv0 = getFloat(s, "lv0", 35)
		cfg.Fan.Lv1 = getFloat(s, "lv1", 40)
		cfg.Fan.Lv2 = getFloat(s, "lv2", 45)
		cfg.Fan.Lv3 = getFloat(s, "lv3", 50)
	}
	// Button actions.
	if s := sections["key"]; s != nil {
		cfg.Key.Click = getStr(s, "click", "slider")
		cfg.Key.Twice = getStr(s, "twice", "switch")
		cfg.Key.Press = getStr(s, "press", "none")
	}
	// Button timing.
	if s := sections["time"]; s != nil {
		cfg.Time.Twice = getFloat(s, "twice", 0.7)
		cfg.Time.Press = getFloat(s, "press", 1.8)
	}
	// Slider/auto-rotate.
	if s := sections["slider"]; s != nil {
		cfg.Slider.Auto = getBool(s, "auto", true)
		cfg.Slider.Time = getFloat(s, "time", 10)
	}
	// OLED display options.
	if s := sections["oled"]; s != nil {
		cfg.OLED.Rotate = getBool(s, "rotate", false)
		cfg.OLED.FTemp = getBool(s, "f-temp", false)
	}

	return cfg
}

// FanSwitch toggles the fan enable/disable state.
func (c *Config) FanSwitch() {
	if atomic.LoadInt64(&c.run) == 1 {
		atomic.StoreInt64(&c.run, 0)
	} else {
		atomic.StoreInt64(&c.run, 1)
	}
}

// FanRunning reports whether the fan is enabled.
func (c *Config) FanRunning() bool {
	return atomic.LoadInt64(&c.run) == 1
}

// SliderNext advances the page counter and returns the next page index.
func (c *Config) SliderNext(pageCount int) int {
	idx := atomic.AddInt64(&c.idx, 1)
	return int(idx) % pageCount
}

// GetFunc maps a button event name to the configured action.
func (c *Config) GetFunc(key string) string {
	switch key {
	case "click":
		return c.Key.Click
	case "twice":
		return c.Key.Twice
	case "press":
		return c.Key.Press
	default:
		return "none"
	}
}

// FanTemp2DC maps a CPU temperature to a PWM duty cycle.
// Lower duty cycle = higher fan speed (0 = 100%, 0.999 = off).
func (c *Config) FanTemp2DC(temp float64) float64 {
	if temp >= c.Fan.Lv3 {
		return 0
	}
	if temp >= c.Fan.Lv2 {
		return 0.25
	}
	if temp >= c.Fan.Lv1 {
		return 0.5
	}
	if temp >= c.Fan.Lv0 {
		return 0.75
	}
	return 0.999
}

// UpdateDisks refreshes the list of attached SATA drives.
func (c *Config) UpdateDisks() {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()

	out := CheckOutput("lsblk | awk '{print $1}'")
	var disks []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "sd") {
			disks = append(disks, line)
		}
	}
	c.disks = disks
}

// GetDisks returns a copy of the current disk list.
func (c *Config) GetDisks() []string {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	return append([]string{}, c.disks...)
}

// --- INI parser (minimal, no external dependency) ---

func parseINI(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sections := make(map[string]map[string]string)
	section := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			section = line[1 : len(line)-1]
			if sections[section] == nil {
				sections[section] = make(map[string]string)
			}
			continue
		}
		if idx := strings.IndexByte(line, '='); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if sections[section] == nil {
				sections[section] = make(map[string]string)
			}
			sections[section][key] = val
		}
	}
	return sections, scanner.Err()
}

func getFloat(s map[string]string, key string, def float64) float64 {
	if v, ok := s[key]; ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getStr(s map[string]string, key, def string) string {
	if v, ok := s[key]; ok && v != "" {
		return v
	}
	return def
}

func getBool(s map[string]string, key string, def bool) bool {
	if v, ok := s[key]; ok {
		switch strings.ToLower(v) {
		case "true", "yes", "1":
			return true
		case "false", "no", "0":
			return false
		}
	}
	return def
}
