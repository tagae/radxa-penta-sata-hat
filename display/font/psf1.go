package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const psf1Magic uint16 = 0x0436

type psf1Header struct {
	Magic    uint16
	Mode     uint8
	Charsize uint8 // glyph height in pixels; width is fixed at 8
}

func parsePSF1(data []byte) (*psf1Header, [][]byte, error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("psf1: header too short")
	}
	var hdr psf1Header
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &hdr); err != nil {
		return nil, nil, fmt.Errorf("psf1: read header: %w", err)
	}
	n := 256
	if hdr.Mode&0x01 != 0 {
		n = 512
	}
	charsize := int(hdr.Charsize)
	glyphs := make([][]byte, n)
	for i := range glyphs {
		off := 4 + i*charsize
		if off+charsize > len(data) {
			return nil, nil, fmt.Errorf("psf1: truncated at glyph %d", i)
		}
		glyphs[i] = data[off : off+charsize]
	}
	return &hdr, glyphs, nil
}
