package font

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const psf2Magic uint32 = 0x864AB572

type psf2Header struct {
	Magic         uint32
	Version       uint32
	HeaderSize    uint32
	Flags         uint32
	NumGlyphs     uint32
	BytesPerGlyph uint32
	Height        uint32
	Width         uint32
}

func parsePSF2(data []byte) (*psf2Header, [][]byte, error) {
	if len(data) < 32 {
		return nil, nil, fmt.Errorf("psf2: header too short")
	}
	var hdr psf2Header
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &hdr); err != nil {
		return nil, nil, fmt.Errorf("psf2: read header: %w", err)
	}
	bpg := int(hdr.BytesPerGlyph)
	glyphs := make([][]byte, hdr.NumGlyphs)
	for i := range glyphs {
		off := int(hdr.HeaderSize) + i*bpg
		if off+bpg > len(data) {
			return nil, nil, fmt.Errorf("psf2: truncated at glyph %d", i)
		}
		glyphs[i] = data[off : off+bpg]
	}
	return &hdr, glyphs, nil
}
