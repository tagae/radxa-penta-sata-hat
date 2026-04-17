package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"gopkg.in/ini.v1"
)

type Config struct {
	Fan    FanConfig    `ini:"fan"`
	Key    KeyConfig    `ini:"key"`
	Time   TimeConfig   `ini:"time"`
	Slider SliderConfig `ini:"slider"`
	OLED   OLEDConfig   `ini:"oled"`

	disks  []string
	diskMu sync.Mutex

	run int64 // atomic fan enable flag (1=on, 0=off)
}

type FanConfig struct {
	OffTemp      int `ini:"off-temp"`
	FullTemp     int `ini:"full-temp"`
	PollInterval int `ini:"poll-interval"`
}

type KeyConfig struct {
	Click string `ini:"click"`
	Twice string `ini:"twice"`
	Press string `ini:"press"`
}

type TimeConfig struct {
	Twice float64 `ini:"twice"`
	Press float64 `ini:"press"`
}

type SliderConfig struct {
	Time float64 `ini:"time"`
}

type OLEDConfig struct {
	Rotate     bool   `ini:"rotate"`
	FTemp      bool   `ini:"f-temp"`
	Font       string `ini:"font"`
	FontMedium string `ini:"font-medium"`
	FontLarge  string `ini:"font-large"`
}

// LoadConfig reads the INI-style configuration file.
// If reading fails, sensible defaults are used.
func LoadConfig(path string) *Config {
	cfg := &Config{
		Fan: FanConfig{OffTemp: 35, FullTemp: 50, PollInterval: 60},
		OLED: OLEDConfig{
			Font:       "Lat15-Terminus12x6.psf.gz",
			FontMedium: "Lat15-Terminus16.psf.gz",
			FontLarge:  "Lat15-Terminus28x14.psf.gz",
		},
		Key:    KeyConfig{Click: "slider", Twice: "switch", Press: "none"},
		Time:   TimeConfig{Twice: 0.7, Press: 1.8},
		Slider: SliderConfig{Time: 10},
	}
	atomic.StoreInt64(&cfg.run, 1)

	if err := ini.MapTo(cfg, path); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v, using defaults\n", err)
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

// FanTemp2DC maps a temperature to a PWM duty cycle.
// Lower duty cycle = higher fan speed (0 = 100%, 1 = off).
// Fan speed ramps linearly from 0% at off-temp to 100% at full-temp.
func (c *Config) FanTemp2DC(temp float64) float64 {
	off, full := float64(c.Fan.OffTemp), float64(c.Fan.FullTemp)
	if temp >= full {
		return 0
	}
	if temp <= off {
		return 1
	}
	return 1 - (temp-off)/(full-off)
}

// UpdateDisks refreshes the list of attached SATA drives.
func (c *Config) UpdateDisks() {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()

	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return
	}
	var disks []string
	for _, e := range entries {
		data, err := os.ReadFile("/sys/block/" + e.Name() + "/device/type")
		if err == nil && strings.TrimSpace(string(data)) == "0" {
			disks = append(disks, e.Name())
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
