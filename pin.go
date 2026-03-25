package main

import (
	"fmt"
	"strconv"
	"strings"
)

// ChipName converts a chip identifier (e.g. "0") to a Linux GPIO chip
// name (e.g. "gpiochip0"). If the input already has the prefix, it is
// returned as-is.
func ChipName(s string) string {
	if _, err := strconv.Atoi(s); err == nil {
		return "gpiochip" + s
	}
	return s
}

// ParsePin converts an Adafruit Blinka-style pin name to a Linux GPIO
// chip name and line offset.
//
// Supported formats:
//   - "D23"       → gpiochip0, line 23  (Raspberry Pi)
//   - "GPIO4_D2"  → gpiochip4, line 26  (Rockchip: bank*32-relative → group*8+pin)
func ParsePin(name string) (chip string, line int) {
	// Raspberry Pi format: D<num>
	if strings.HasPrefix(name, "D") {
		num, _ := strconv.Atoi(name[1:])
		return "gpiochip0", num
	}

	// Rockchip format: GPIO<bank>_<group><pin>
	// Group: A=0, B=1, C=2, D=3
	// Line within chip = group*8 + pin
	if strings.HasPrefix(name, "GPIO") {
		rest := name[4:] // e.g. "4_D2"
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 && len(parts[1]) >= 2 {
			bank, _ := strconv.Atoi(parts[0])
			group := int(parts[1][0] - 'A')
			pin, _ := strconv.Atoi(parts[1][1:])
			return fmt.Sprintf("gpiochip%d", bank), group*8 + pin
		}
	}

	return "gpiochip0", 0
}

// ParseI2CBus converts an Adafruit Blinka-style SCL pin name to a
// Linux I2C device path.
//
// Supported formats:
//   - "SCL"       → /dev/i2c-1  (Raspberry Pi default)
//   - "I2C7_SCL"  → /dev/i2c-7  (Rockchip)
func ParseI2CBus(scl string) string {
	if strings.HasPrefix(scl, "I2C") {
		parts := strings.SplitN(scl[3:], "_", 2)
		return "/dev/i2c-" + parts[0]
	}
	return "/dev/i2c-1"
}
