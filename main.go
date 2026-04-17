package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"radxa-penta/display"
	"radxa-penta/sys"

	"github.com/spf13/cobra"
)

func main() {
	var configPath string
	var fontDir string

	root := &cobra.Command{
		Use:           "radxa-penta",
		Short:         "Fan control and OLED display for the Radxa Penta SATA HAT",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", "radxa-penta.conf", "path to config file")
	root.PersistentFlags().StringVar(&fontDir, "fonts", "/usr/share/consolefonts", "directory containing PSF fonts")

	root.AddCommand(
		serveCmd(&configPath, &fontDir),
		statusCmd(&configPath),
		fanCmd(&configPath),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd(configPath, fontDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the fan and display service",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(*configPath)
			cfg.UpdateDisks()

			fan := NewFan(cfg)

			// Try to initialize the OLED display (top board). If it fails
			// (no I2C bus, no display attached), run in headless mode with
			// fan control only.
			oledCfg := display.Config{
				Rotate:     cfg.OLED.Rotate,
				Font:       cfg.OLED.Font,
				FontMedium: cfg.OLED.FontMedium,
				FontLarge:  cfg.OLED.FontLarge,
				SliderTime: cfg.Slider.Time,
			}
			oled, err := display.New(oledCfg, *fontDir)
			topBoard := err == nil
			if err != nil {
				fmt.Fprintf(os.Stderr, "OLED init failed (headless mode): %v\n", err)
			}

			if topBoard {
				eventCh := make(chan string, 10)
				var mu sync.Mutex
				genPages := func() []display.Page { return buildPages(cfg) }

				actions := map[string]func(){
					"none":     func() {},
					"slider":   func() { oled.Slider(&mu, genPages) },
					"switch":   func() { cfg.FanSwitch() },
					"reboot":   func() { syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART) },
					"poweroff": func() { syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF) },
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
				go oled.AutoSlider(&mu, genPages)

				// Fan thermal control.
				go fan.Run()

				// Wait for termination signal.
				sig := make(chan os.Signal, 1)
				signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
				<-sig

				oled.Goodbye()
				oled.Close()
			} else {
				// Headless: only run fan control (blocks).
				fan.Run()
			}
			return nil
		},
	}
}

func fanCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "fan <speed>",
		Short: "Set fan speed (0–100)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			speed, err := strconv.Atoi(args[0])
			if err != nil || speed < 0 || speed > 100 {
				return fmt.Errorf("speed must be an integer between 0 and 100")
			}
			cfg := LoadConfig(*configPath)
			fan := NewFan(cfg)
			if err := fan.initPin(); err != nil {
				return err
			}
			dc := 1.0 - float64(speed)/100.0
			fan.pin.Write(dc)
			fmt.Printf("Fan speed set to %d%%\n", speed)
			// Software PWM requires the process to stay alive to keep
			// generating pulses; block until the user interrupts.
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			return nil
		},
	}
}

func statusCmd(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print current board status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(*configPath)
			cfg.UpdateDisks()

			fmt.Println(sys.GetUptime())
			fmt.Println(sys.GetIP())
			fmt.Println(sys.GetMemory())
			fmt.Println(sys.GetCPUTemp(cfg.OLED.FTemp))

			disks := cfg.GetDisks()
			temps := sys.DriveTemps(disks)
			for _, disk := range disks {
				mp := sys.MountpointOf("/dev/" + disk)
				usage := sys.DiskUsagePct(mp)
				if t, ok := temps[disk]; ok {
					fmt.Printf("Drive %s: %.0f°C  Disk: %s\n", disk, t, usage)
				} else {
					fmt.Printf("Drive %s: N/A  Disk: %s\n", disk, usage)
				}
			}

			dc := cfg.FanTemp2DC(sys.DriveTemp(disks))
			fmt.Printf("Fan: %d%%\n", int((1-dc)*100))
			return nil
		},
	}
}
