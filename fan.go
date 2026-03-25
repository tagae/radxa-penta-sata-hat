package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// FanPin is the interface for writing a PWM duty cycle.
type FanPin interface {
	Write(duty float64)
}

// --- Hardware PWM via sysfs ---

type HardwarePWM struct {
	dir         string
	periodValue int
}

func NewHardwarePWM(chip string) (*HardwarePWM, error) {
	if _, err := strconv.Atoi(chip); err == nil {
		chip = "pwmchip" + chip
	}

	pwm := &HardwarePWM{
		dir: fmt.Sprintf("/sys/class/pwm/%s/pwm0/", chip),
	}

	exportPath := fmt.Sprintf("/sys/class/pwm/%s/export", chip)
	if err := os.WriteFile(exportPath, []byte("0"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: init pwm error: %v\n", err)
	}

	return pwm, nil
}

func (p *HardwarePWM) SetPeriodUS(us int) error {
	p.periodValue = us * 1000
	return os.WriteFile(filepath.Join(p.dir, "period"), []byte(strconv.Itoa(p.periodValue)), 0644)
}

func (p *HardwarePWM) Enable(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return os.WriteFile(filepath.Join(p.dir, "enable"), []byte(v), 0644)
}

func (p *HardwarePWM) Write(duty float64) {
	if p.periodValue == 0 {
		return
	}
	dc := strconv.Itoa(int(float64(p.periodValue) * duty))
	os.WriteFile(filepath.Join(p.dir, "duty_cycle"), []byte(dc), 0644)
}

// --- Software PWM via GPIO bit-banging ---

type GpioPWM struct {
	line    *gpiocdev.Line
	mu      sync.Mutex
	onTime  float64
	offTime float64
	period  float64
}

func NewGpioPWM(periodS float64) (*GpioPWM, error) {
	chipName := ChipName(os.Getenv("FAN_CHIP"))
	lineNum, err := strconv.Atoi(os.Getenv("FAN_LINE"))
	if err != nil {
		return nil, fmt.Errorf("invalid FAN_LINE: %v", err)
	}

	line, err := gpiocdev.RequestLine(chipName, lineNum, gpiocdev.AsOutput(0))
	if err != nil {
		return nil, fmt.Errorf("request fan GPIO %s/%d: %w", chipName, lineNum, err)
	}

	g := &GpioPWM{
		line:    line,
		onTime:  periodS / 2,
		offTime: periodS / 2,
		period:  periodS,
	}
	go g.run()
	return g, nil
}

func (g *GpioPWM) run() {
	for {
		g.mu.Lock()
		on, off := g.onTime, g.offTime
		g.mu.Unlock()

		g.line.SetValue(1)
		time.Sleep(time.Duration(on * float64(time.Second)))
		g.line.SetValue(0)
		time.Sleep(time.Duration(off * float64(time.Second)))
	}
}

func (g *GpioPWM) Write(duty float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.offTime = duty * g.period
	g.onTime = g.period - g.offTime
}

// --- Fan controller ---

type Fan struct {
	cfg    *Config
	pin    FanPin
	lastDC float64

	cacheMu sync.Mutex
	cacheDC float64
	cacheAt time.Time
}

func NewFan(cfg *Config) *Fan {
	return &Fan{cfg: cfg, lastDC: -1}
}

func (f *Fan) getDC() float64 {
	if !f.cfg.FanRunning() {
		return 0.999
	}

	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()

	if time.Since(f.cacheAt) > 60*time.Second {
		f.cacheAt = time.Now()
		f.cacheDC = f.cfg.FanTemp2DC(ReadTemp())
	}
	return f.cacheDC
}

func (f *Fan) changeDC(dc float64) {
	if dc != f.lastDC {
		f.lastDC = dc
		f.pin.Write(dc)
	}
}

// Run is the main fan control loop. It initialises the appropriate PWM
// output and then adjusts the duty cycle every second.
func (f *Fan) Run() {
	if os.Getenv("HARDWARE_PWM") == "1" {
		pwm, err := NewHardwarePWM(os.Getenv("PWMCHIP"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "hardware PWM init: %v\n", err)
			return
		}
		pwm.SetPeriodUS(40)
		pwm.Enable(true)
		f.pin = pwm
	} else {
		gpio, err := NewGpioPWM(0.025)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GPIO PWM init: %v\n", err)
			return
		}
		f.pin = gpio
	}

	for {
		f.changeDC(f.getDC())
		time.Sleep(1 * time.Second)
	}
}
