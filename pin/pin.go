package pin

import "strconv"

// ChipName converts a chip identifier (e.g. "0") to a Linux GPIO chip
// name (e.g. "gpiochip0"). If the input already has the prefix, it is
// returned as-is.
func ChipName(s string) string {
	if _, err := strconv.Atoi(s); err == nil {
		return "gpiochip" + s
	}
	return s
}

// ParsePin converts a Raspberry Pi Blinka-style pin name (e.g. "D23")
// to a GPIO chip name and line offset.
func ParsePin(name string) (chip string, line int) {
	num, _ := strconv.Atoi(name[1:]) // strip leading "D"
	return "gpiochip0", num
}

// I2CBus returns the Raspberry Pi I2C device path.
func I2CBus() string {
	return "/dev/i2c-1"
}
