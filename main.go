package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	cfg := LoadConfig("/etc/rockpi-penta.conf")
	cfg.UpdateDisks()

	fan := NewFan(cfg)

	// Try to initialize the OLED display (top board). If it fails
	// (no I2C bus, no display attached), run in headless mode with
	// fan control only.
	display, err := NewOLED(cfg)
	topBoard := err == nil
	if err != nil {
		fmt.Fprintf(os.Stderr, "OLED init failed (headless mode): %v\n", err)
	}

	if topBoard {
		display.Welcome()

		eventCh := make(chan string, 10)
		var mu sync.Mutex

		actions := map[string]func(){
			"none":     func() {},
			"slider":   func() { display.Slider(&mu) },
			"switch":   func() { cfg.FanSwitch() },
			"reboot":   func() { CheckCall("reboot") },
			"poweroff": func() { CheckCall("poweroff") },
		}

		// Receive button events and dispatch actions.
		go func() {
			for key := range eventCh {
				fn := cfg.GetFunc(key)
				if action, ok := actions[fn]; ok {
					action()
				}
			}
		}()

		// Watch the hardware button.
		go WatchKey(cfg, eventCh)

		// Auto-rotate OLED pages.
		go display.AutoSlider(&mu)

		// Fan thermal control.
		go fan.Run()

		// Wait for termination signal.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Println("GoodBye ~")
		display.Goodbye()
		display.Close()
	} else {
		// Headless: only run fan control (blocks).
		fan.Run()
	}
}
