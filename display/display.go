package display

import (
	"fmt"
	"image"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"radxa-penta/display/font"
	"radxa-penta/pin"

	"github.com/warthog618/go-gpiocdev"
)

// Item is a text element to render at a given position.
type Item struct {
	X, Y int
	Text string
}

// Page is a collection of items rendered together as one screen.
type Page []Item

// Display manages the SSD1306 display.
type Display struct {
	dev        *ssd1306
	cfg        Config
	smallFont  *font.Font // 3-row pages
	mediumFont *font.Font // 2-row pages
	largeFont  *font.Font // 1-row pages
	resetPin   *gpiocdev.Line
	width      int
	height     int
	pageIdx    int64 // atomic page counter; starts at -1 so first advance yields 0
}

// New resets the display hardware, initializes the SSD1306 over
// I2C, and loads the PSF fonts from fontDir.
func New(cfg Config, fontDir string) (*Display, error) {
	// Reset the display via GPIO.
	resetChip, resetLine := pin.ParsePin(os.Getenv("OLED_RESET"))
	rst, err := gpiocdev.RequestLine(resetChip, resetLine, gpiocdev.AsOutput(1))
	if err != nil {
		return nil, fmt.Errorf("reset GPIO: %w", err)
	}
	rst.SetValue(1)
	time.Sleep(time.Millisecond)
	rst.SetValue(0)
	time.Sleep(10 * time.Millisecond)
	rst.SetValue(1)
	time.Sleep(10 * time.Millisecond)

	// Open I2C and init SSD1306.
	i2cDev := pin.I2CBus()
	dev, err := newSSD1306(i2cDev, 128, 32)
	if err != nil {
		rst.Close()
		return nil, fmt.Errorf("SSD1306: %w", err)
	}
	dev.clear() //nolint:errcheck

	// Load PSF fonts; medium and large fall back to small if unset or missing.
	small, err := font.Load(fontDir + "/" + cfg.Font)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	loadOpt := func(name string) *font.Font {
		if name == "" {
			return small
		}
		if f, err := font.Load(fontDir + "/" + name); err == nil {
			return f
		}
		return small
	}
	medium := loadOpt(cfg.FontMedium)
	large := loadOpt(cfg.FontLarge)

	return &Display{
		dev:        dev,
		cfg:        cfg,
		smallFont:  small,
		mediumFont: medium,
		largeFont:  large,
		resetPin:   rst,
		width:      128,
		height:     32,
		pageIdx:    -1,
	}, nil
}

// newImage returns a blank 128x32 grayscale image.
func (d *Display) newImage() *image.Gray {
	return image.NewGray(image.Rect(0, 0, d.width, d.height))
}

// show sends the image to the display, optionally rotating 180°.
func (d *Display) show(img *image.Gray) {
	out := img
	if d.cfg.Rotate {
		out = rotate180(img)
	}
	d.dev.display(imageToSSD1306(out, d.width, d.height)) //nolint:errcheck
}

// Goodbye shows the shutdown screen for 2 seconds, then clears.
func (d *Display) Goodbye() {
	img := d.newImage()
	y := (d.height - d.largeFont.Height) / 2
	d.largeFont.DrawText(img, 0, y, "Goodbye ~")
	d.show(img)
	time.Sleep(2 * time.Second)
	d.dev.clear() //nolint:errcheck
}

// Close releases hardware resources.
func (d *Display) Close() {
	d.dev.close() //nolint:errcheck
	d.resetPin.Close()
}

// Slider renders the next page returned by genPages (thread-safe).
func (d *Display) Slider(mu *sync.Mutex, genPages func() []Page) {
	mu.Lock()
	defer mu.Unlock()

	pages := genPages()
	idx := int(atomic.AddInt64(&d.pageIdx, 1)) % len(pages)
	page := pages[idx]

	var fnt *font.Font
	switch {
	case len(page) == 1:
		fnt = d.largeFont
	case len(page) == 2:
		fnt = d.mediumFont
	default:
		fnt = d.smallFont
	}

	img := d.newImage()
	for _, item := range page {
		fnt.DrawText(img, item.X, item.Y, item.Text)
	}
	d.show(img)
}

// AutoSlider continuously rotates through pages at the configured interval.
func (d *Display) AutoSlider(mu *sync.Mutex, genPages func() []Page) {
	for {
		d.Slider(mu, genPages)
		time.Sleep(time.Duration(d.cfg.SliderTime * float64(time.Second)))
	}
}
