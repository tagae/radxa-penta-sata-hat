package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/warthog618/go-gpiocdev"
	"radxa-penta/pin"
	"radxa-penta/sys"
)

// fanPeriod is the PWM cycle duration in seconds.
const fanPeriod = 0.013

type GpioPWM struct {
	line    *gpiocdev.Line
	mu      sync.Mutex
	onTime  float64
	offTime float64
	period  float64
}

func NewGpioPWM(periodS float64) (*GpioPWM, error) {
	chipName := pin.ChipName(os.Getenv("FAN_CHIP"))
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

		if err := g.line.SetValue(1); err != nil {
			fmt.Fprintf(os.Stderr, "fan GPIO set high: %v\n", err)
		}
		time.Sleep(time.Duration(on * float64(time.Second)))
		if err := g.line.SetValue(0); err != nil {
			fmt.Fprintf(os.Stderr, "fan GPIO set low: %v\n", err)
		}
		time.Sleep(time.Duration(off * float64(time.Second)))
	}
}

func (g *GpioPWM) Write(duty float64) {
	g.mu.Lock()
	g.offTime = duty * g.period
	g.onTime = g.period - g.offTime
	g.mu.Unlock()
}

// --- Fan controller ---

type Fan struct {
	cfg    *Config
	pin    *GpioPWM
	lastDC float64
}

func NewFan(cfg *Config) *Fan {
	return &Fan{cfg: cfg, lastDC: -1}
}

func (f *Fan) getDC() float64 {
	if !f.cfg.FanRunning() {
		return 1
	}
	return f.cfg.FanTemp2DC(sys.DriveTemp(f.cfg.GetDisks()))
}

func (f *Fan) changeDC(dc float64) {
	if dc != f.lastDC {
		f.lastDC = dc
		f.pin.Write(dc)
	}
}

func (f *Fan) initPin() error {
	gpio, err := NewGpioPWM(fanPeriod)
	if err != nil {
		return fmt.Errorf("GPIO PWM init: %w", err)
	}
	f.pin = gpio
	return nil
}

// Run is the main fan control loop. It initializes the PWM output and
// then adjusts the duty cycle every fanInterval based on drive temperature.
func (f *Fan) Run() {
	if err := f.initPin(); err != nil {
		fmt.Fprintf(os.Stderr, "fan init: %v\n", err)
		return
	}

	for {
		f.changeDC(f.getDC())
		time.Sleep(time.Duration(f.cfg.Fan.PollInterval) * time.Second)
	}
}
