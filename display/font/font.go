package font

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Font is a bitmap font ready for rendering.
type Font struct {
	Width      int
	Height     int
	glyphs     [][]byte
	unicodeMap map[rune]int // rune → glyph index; nil for PSF1
}

// Load reads a PSF1 or PSF2 font file (plain or gzip-compressed).
func Load(path string) (*Font, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("psf gzip: %w", err)
		}
		defer gz.Close()
		r = gz
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("psf read: %w", err)
	}
	return parse(data)
}

func parse(data []byte) (*Font, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("psf: file too short")
	}
	switch {
	case binary.LittleEndian.Uint16(data) == psf1Magic:
		return fromPSF1(data)
	case binary.LittleEndian.Uint32(data) == psf2Magic:
		return fromPSF2(data)
	default:
		return nil, fmt.Errorf("psf: unrecognised magic bytes")
	}
}

func fromPSF1(data []byte) (*Font, error) {
	hdr, glyphs, err := parsePSF1(data)
	if err != nil {
		return nil, err
	}
	return &Font{Width: 8, Height: int(hdr.Charsize), glyphs: glyphs}, nil
}

func fromPSF2(data []byte) (*Font, error) {
	hdr, glyphs, err := parsePSF2(data)
	if err != nil {
		return nil, err
	}
	var unicodeMap map[rune]int
	if hdr.Flags&0x01 != 0 {
		tableStart := int(hdr.HeaderSize) + int(hdr.NumGlyphs)*int(hdr.BytesPerGlyph)
		unicodeMap = parseUnicodeTable(data[tableStart:], int(hdr.NumGlyphs))
	}
	return &Font{
		Width:      int(hdr.Width),
		Height:     int(hdr.Height),
		glyphs:     glyphs,
		unicodeMap: unicodeMap,
	}, nil
}

// parseUnicodeTable builds a rune→glyph-index map from a PSF2 unicode table.
// Each glyph's entry is a sequence of UTF-8 runes, terminated by 0xFF.
// Ligature sequences (introduced by 0xFE) are skipped.
func parseUnicodeTable(table []byte, numGlyphs int) map[rune]int {
	m := make(map[rune]int)
	glyph, i := 0, 0
	for i < len(table) && glyph < numGlyphs {
		for i < len(table) && table[i] != 0xFF {
			if table[i] == 0xFE {
				i++
				continue
			}
			r, size := utf8.DecodeRune(table[i:])
			if r != utf8.RuneError {
				if _, exists := m[r]; !exists {
					m[r] = glyph
				}
			}
			i += size
		}
		i++ // consume 0xFF terminator
		glyph++
	}
	return m
}

// DrawText blits text onto img with the top-left corner at (x, y).
func (f *Font) DrawText(img *image.Gray, x, y int, text string) {
	stride := (f.Width + 7) / 8
	bounds := img.Bounds()
	cx := x
	for _, ch := range text {
		idx := f.glyphIndex(ch)
		if idx >= 0 {
			glyph := f.glyphs[idx]
			for row := 0; row < f.Height; row++ {
				py := y + row
				if py < bounds.Min.Y || py >= bounds.Max.Y {
					continue
				}
				for col := 0; col < f.Width; col++ {
					px := cx + col
					if px < bounds.Min.X || px >= bounds.Max.X {
						continue
					}
					if glyph[row*stride+col/8]>>(7-uint(col%8))&1 == 1 {
						img.SetGray(px, py, color.Gray{Y: 255})
					}
				}
			}
		}
		cx += f.Width
	}
}

func (f *Font) glyphIndex(ch rune) int {
	if f.unicodeMap != nil {
		if idx, ok := f.unicodeMap[ch]; ok {
			return idx
		}
		return -1
	}
	// PSF1: direct Latin-1 code-point indexing.
	idx := int(ch)
	if idx >= 0 && idx < len(f.glyphs) {
		return idx
	}
	return -1
}
