package display

// Config holds the display configuration for an OLED.
type Config struct {
	Rotate     bool
	Font       string
	FontMedium string
	FontLarge  string
	SliderTime float64
}
