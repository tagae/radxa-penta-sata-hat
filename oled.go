package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"sync"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/warthog618/go-gpiocdev"
	"golang.org/x/image/font"
)

// OLED manages the SSD1306 display.
type OLED struct {
	dev      *SSD1306
	cfg      *Config
	ttFont   *truetype.Font
	resetPin *gpiocdev.Line
	width    int
	height   int
	ascents  map[int]int // font size → pixel ascent
}

// NewOLED resets the display hardware, initialises the SSD1306 over
// I2C, and loads the TTF font.
func NewOLED(cfg *Config) (*OLED, error) {
	// Reset the display via GPIO.
	resetChip, resetLine := ParsePin(os.Getenv("OLED_RESET"))
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
	i2cDev := ParseI2CBus(os.Getenv("SCL"))
	dev, err := NewSSD1306(i2cDev, 128, 32)
	if err != nil {
		rst.Close()
		return nil, fmt.Errorf("SSD1306: %w", err)
	}
	dev.Clear()

	// Load TTF font.
	fontDir := os.Getenv("FONT_DIR")
	if fontDir == "" {
		fontDir = "fonts"
	}
	fontData, err := os.ReadFile(fontDir + "/DejaVuSansMono-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}
	ttFont, err := truetype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}

	// Pre-compute pixel ascents for each font size used by the UI.
	ascents := make(map[int]int)
	for _, size := range []int{10, 11, 12, 14} {
		face := truetype.NewFace(ttFont, &truetype.Options{
			Size:    float64(size),
			DPI:     72,
			Hinting: font.HintingFull,
		})
		ascents[size] = face.Metrics().Ascent.Ceil()
	}

	return &OLED{
		dev:      dev,
		cfg:      cfg,
		ttFont:   ttFont,
		resetPin: rst,
		width:    128,
		height:   32,
		ascents:  ascents,
	}, nil
}

// newImage returns a blank 128x32 grayscale image.
func (o *OLED) newImage() *image.Gray {
	return image.NewGray(image.Rect(0, 0, o.width, o.height))
}

// drawText renders text onto img. (x, y) is the top-left corner of the
// text bounding box, matching PIL's draw.text() coordinate convention.
func (o *OLED) drawText(img *image.Gray, x, y int, text string, fontSize int) {
	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(o.ttFont)
	ctx.SetFontSize(float64(fontSize))
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(color.Gray{Y: 255}))

	ascent := o.ascents[fontSize]
	pt := freetype.Pt(x, y+ascent)
	ctx.DrawString(text, pt)
}

// show sends the image to the display, optionally rotating 180°.
func (o *OLED) show(img *image.Gray) {
	out := img
	if o.cfg.OLED.Rotate {
		out = Rotate180(img)
	}
	o.dev.Display(ImageToSSD1306(out, o.width, o.height))
}

// Welcome shows the startup splash screen.
func (o *OLED) Welcome() {
	img := o.newImage()
	o.drawText(img, 0, 0, "ROCKPi SATA HAT", 14)
	o.drawText(img, 32, 16, "Loading...", 12)
	o.show(img)
}

// Goodbye shows the shutdown screen for 2 seconds, then clears.
func (o *OLED) Goodbye() {
	img := o.newImage()
	o.drawText(img, 32, 8, "Good Bye ~", 14)
	o.show(img)
	time.Sleep(2 * time.Second)
	o.dev.Clear()
}

// Close releases hardware resources.
func (o *OLED) Close() {
	o.dev.Close()
	o.resetPin.Close()
}

// --- Display pages ---

type textItem struct {
	x, y     int
	text     string
	fontSize int
}

func (o *OLED) genPages() [][]textItem {
	page0 := []textItem{
		{0, -2, GetInfo("up"), 11},
		{0, 10, GetCPUTemp(o.cfg.OLED.FTemp), 11},
		{0, 21, GetInfo("ip"), 11},
	}

	page1 := []textItem{
		{0, 2, GetInfo("cpu"), 12},
		{0, 18, GetInfo("men"), 12},
	}

	page2 := o.diskPage()

	return [][]textItem{page0, page1, page2}
}

func (o *OLED) diskPage() []textItem {
	keys, vals := GetDiskInfo(o.cfg)
	if len(keys) == 0 {
		return []textItem{{0, 2, "Disk: N/A", 14}}
	}

	text1 := fmt.Sprintf("Disk: %s %s", keys[0], vals[0])

	switch len(keys) {
	case 5:
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], vals[1], keys[2], vals[2])
		text3 := fmt.Sprintf("%s %s  %s %s", keys[3], vals[3], keys[4], vals[4])
		return []textItem{
			{0, -2, text1, 11},
			{0, 10, text2, 11},
			{0, 21, text3, 11},
		}
	case 3:
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], vals[1], keys[2], vals[2])
		return []textItem{
			{0, 2, text1, 12},
			{0, 18, text2, 12},
		}
	default:
		return []textItem{{0, 2, text1, 14}}
	}
}

// Slider renders the next OLED page (thread-safe).
func (o *OLED) Slider(mu *sync.Mutex) {
	mu.Lock()
	defer mu.Unlock()

	pages := o.genPages()
	idx := o.cfg.SliderNext(len(pages))

	img := o.newImage()
	for _, item := range pages[idx] {
		o.drawText(img, item.x, item.y, item.text, item.fontSize)
	}
	o.show(img)
}

// AutoSlider continuously rotates through display pages at the
// configured interval. If auto-rotate is disabled, it displays one
// page and returns.
func (o *OLED) AutoSlider(mu *sync.Mutex) {
	if o.cfg.Slider.Auto {
		for {
			o.Slider(mu)
			time.Sleep(time.Duration(o.cfg.Slider.Time * float64(time.Second)))
		}
	} else {
		o.Slider(mu)
	}
}
