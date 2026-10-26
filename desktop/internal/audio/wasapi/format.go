package wasapi

import (
	"encoding/binary"
	"fmt"
)

// SampleKind adalah representasi sampel yang dikenali konverter.
type SampleKind int

// Jenis sampel yang dikenali.
const (
	KindUnknown SampleKind = iota
	KindInt16
	KindInt24
	KindInt32
	KindFloat32
)

func (k SampleKind) String() string {
	switch k {
	case KindInt16:
		return "int16"
	case KindInt24:
		return "int24"
	case KindInt32:
		return "int32"
	case KindFloat32:
		return "float32"
	default:
		return "unknown"
	}
}

// Ukuran struct dari mmreg.h. Header memakai pshpack1.h (mmreg.h:14) sehingga
// tanpa padding: WAVEFORMATEX = 18 byte, WAVEFORMATEXTENSIBLE = 18 + 2 + 4 + 16 = 40 byte.
const (
	waveformatexSize         = 18
	waveformatextensibleSize = 40
)

// Format adalah isi WAVEFORMATEX / WAVEFORMATEXTENSIBLE yang sudah di-parse.
type Format struct {
	Tag            uint16 // wFormatTag mentah
	EffectiveTag   uint16 // Tag setelah SubFormat dibuka bila extensible; 0 bila SubFormat tidak dikenal
	Channels       uint16
	SampleRate     uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16 // ukuran kontainer per sampel
	ValidBits      uint16
	ChannelMask    uint32
	Extensible     bool
	Kind           SampleKind
}

func (f Format) String() string {
	ext := ""
	if f.Extensible {
		ext = fmt.Sprintf(" extensible(mask=0x%X valid=%d)", f.ChannelMask, f.ValidBits)
	}
	return fmt.Sprintf("%s %dch %dHz align=%d container=%dbit tag=0x%04X%s",
		f.Kind, f.Channels, f.SampleRate, f.BlockAlign, f.BitsPerSample, f.EffectiveTag, ext)
}

// ParseFormat mem-parse byte mentah WAVEFORMATEX (little-endian, tanpa padding).
func ParseFormat(b []byte) (Format, error) {
	if len(b) < waveformatexSize {
		return Format{}, fmt.Errorf("format: panjang %d kurang dari %d byte", len(b), waveformatexSize)
	}
	le := binary.LittleEndian
	f := Format{
		Tag:            le.Uint16(b[0:]),
		Channels:       le.Uint16(b[2:]),
		SampleRate:     le.Uint32(b[4:]),
		AvgBytesPerSec: le.Uint32(b[8:]),
		BlockAlign:     le.Uint16(b[12:]),
		BitsPerSample:  le.Uint16(b[14:]),
	}
	cb := le.Uint16(b[16:])
	f.EffectiveTag = f.Tag
	f.ValidBits = f.BitsPerSample
	if f.Tag == WaveFormatExtensible {
		if cb < 22 || len(b) < waveformatextensibleSize {
			return Format{}, fmt.Errorf("format: extensible butuh cbSize>=22 dan %d byte (cbSize=%d, panjang=%d)",
				waveformatextensibleSize, cb, len(b))
		}
		f.Extensible = true
		f.ValidBits = le.Uint16(b[18:])
		f.ChannelMask = le.Uint32(b[20:])
		f.EffectiveTag = subFormatTag(b[24:40])
	}
	f.Kind = kindOf(f.EffectiveTag, f.BitsPerSample)
	return f, nil
}

// subFormatTag membuka SubFormat GUID bentuk standar {tag-0000-0010-8000-00aa00389b71}
// (makro DEFINE_WAVEFORMATEX_GUID, mmreg.h:2005-2013). Mengembalikan 0 bila bukan bentuk standar.
func subFormatTag(g []byte) uint16 {
	le := binary.LittleEndian
	d1 := le.Uint32(g[0:])
	d2 := le.Uint16(g[4:])
	d3 := le.Uint16(g[6:])
	tail := [8]byte{0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71}
	var d4 [8]byte
	copy(d4[:], g[8:16])
	if d1 > 0xffff || d2 != 0 || d3 != 0x0010 || d4 != tail {
		return 0
	}
	return uint16(d1)
}

func kindOf(tag, bits uint16) SampleKind {
	switch {
	case tag == WaveFormatPCM && bits == 16:
		return KindInt16
	case tag == WaveFormatPCM && bits == 24:
		return KindInt24
	case tag == WaveFormatPCM && bits == 32:
		return KindInt32
	case tag == WaveFormatIEEEFloat && bits == 32:
		return KindFloat32
	default:
		return KindUnknown
	}
}
