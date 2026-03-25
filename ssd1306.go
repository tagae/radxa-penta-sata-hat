package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"golang.org/x/sys/unix"
)

const (
	ssd1306Addr    = 0x3C
	i2cSlaveIoctl  = 0x0703
	ssd1306CmdMode = 0x00
	ssd1306DatMode = 0x40
)

// SSD1306 drives a SSD1306 OLED display over I2C.
type SSD1306 struct {
	file   *os.File
	width  int
	height int
}

// NewSSD1306 opens the I2C device, sets the slave address, and
// initialises the SSD1306 controller.
func NewSSD1306(devPath string, width, height int) (*SSD1306, error) {
	f, err := os.OpenFile(devPath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", devPath, err)
	}

	if err := unix.IoctlSetInt(int(f.Fd()), i2cSlaveIoctl, ssd1306Addr); err != nil {
		f.Close()
		return nil, fmt.Errorf("ioctl I2C_SLAVE: %w", err)
	}

	d := &SSD1306{file: f, width: width, height: height}
	if err := d.init(); err != nil {
		f.Close()
		return nil, fmt.Errorf("SSD1306 init: %w", err)
	}
	return d, nil
}

// cmd sends one or more single-byte commands to the controller.
func (d *SSD1306) cmd(cmds ...byte) error {
	buf := [2]byte{ssd1306CmdMode, 0}
	for _, c := range cmds {
		buf[1] = c
		if _, err := d.file.Write(buf[:]); err != nil {
			return err
		}
	}
	return nil
}

func (d *SSD1306) init() error {
	comPins := byte(0x02) // 128x32
	if d.height == 64 {
		comPins = 0x12
	}

	return d.cmd(
		0xAE,       // display off
		0xD5, 0x80, // set display clock divider
		0xA8, byte(d.height-1), // set multiplex ratio
		0xD3, 0x00, // set display offset
		0x40,       // set start line = 0
		0x8D, 0x14, // enable charge pump
		0x20, 0x00, // horizontal addressing mode
		0xA1,          // segment remap (column 127 = SEG0)
		0xC8,          // COM output scan direction (remapped)
		0xDA, comPins, // COM pins hardware configuration
		0x81, 0xCF, // set contrast
		0xD9, 0xF1, // set pre-charge period
		0xDB, 0x40, // set VCOMH deselect level
		0xA4, // display contents from RAM
		0xA6, // normal display (not inverted)
		0xAF, // display on
	)
}

// Display sends a raw framebuffer to the display. The buffer must be
// width*(height/8) bytes long, in SSD1306 page format (each byte
// represents 8 vertical pixels, LSB on top).
func (d *SSD1306) Display(buf []byte) error {
	// Set column and page addressing window.
	if err := d.cmd(0x21, 0x00, byte(d.width-1)); err != nil {
		return err
	}
	if err := d.cmd(0x22, 0x00, byte(d.height/8-1)); err != nil {
		return err
	}

	// Prefix data-mode byte and send.
	data := make([]byte, len(buf)+1)
	data[0] = ssd1306DatMode
	copy(data[1:], buf)
	_, err := d.file.Write(data)
	return err
}

// Clear blanks the display.
func (d *SSD1306) Clear() error {
	return d.Display(make([]byte, d.width*d.height/8))
}

// Close turns the display off and releases the I2C file descriptor.
func (d *SSD1306) Close() error {
	d.cmd(0xAE)
	return d.file.Close()
}

// ImageToSSD1306 converts a grayscale image to the SSD1306 page-mode
// framebuffer format. Pixels with Y > 127 are considered "on".
func ImageToSSD1306(img *image.Gray, width, height int) []byte {
	buf := make([]byte, width*(height/8))
	for page := 0; page < height/8; page++ {
		for x := 0; x < width; x++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				y := page*8 + bit
				if img.GrayAt(x, y).Y > 127 {
					b |= 1 << uint(bit)
				}
			}
			buf[page*width+x] = b
		}
	}
	return buf
}

// Rotate180 returns a 180-degree rotated copy of the image.
func Rotate180(src *image.Gray) *image.Gray {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewGray(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.SetGray(x, y, color.Gray{Y: src.GrayAt(w-1-x, h-1-y).Y})
		}
	}
	return dst
}
