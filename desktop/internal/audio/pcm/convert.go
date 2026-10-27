// Package pcm berisi logika audio murni untuk pipeline capture desktop:
// konversi format, downmix, resampler polifase, framer, ring buffer, dan
// statistik. Tanpa COM, tanpa CGO, tanpa Wails (ADR-0001), dan tanpa tag build,
// sehingga seluruh tesnya berjalan di CI.
//
// Konvensi API: fungsi "Append..." dan Process memakai gaya append(dst, ...)
// agar pemanggil bisa memakai ulang buffer (dst[:0]) tanpa alokasi per paket.
package pcm

import (
	"encoding/binary"
	"fmt"
	"math"
	"slices"
)

// Encoding adalah representasi sampel pada byte mentah (little-endian).
type Encoding int

// Encoding yang dikenali. Padanan di paket wasapi: SampleKind.
const (
	EncodingUnknown Encoding = iota
	EncodingInt16
	EncodingInt24
	EncodingInt32
	EncodingFloat32
)

// BytesPerSample mengembalikan ukuran satu sampel dalam byte (0 bila tidak dikenal).
func (e Encoding) BytesPerSample() int {
	switch e {
	case EncodingInt16:
		return 2
	case EncodingInt24:
		return 3
	case EncodingInt32, EncodingFloat32:
		return 4
	default:
		return 0
	}
}

func (e Encoding) String() string {
	switch e {
	case EncodingInt16:
		return "int16"
	case EncodingInt24:
		return "int24"
	case EncodingInt32:
		return "int32"
	case EncodingFloat32:
		return "float32"
	default:
		return "unknown"
	}
}

// AppendDecoded mengubah byte mentah (sampel interleaved, little-endian) menjadi
// float32 pada rentang nominal [-1, 1) dan menambahkannya ke dst.
// Untuk float32: NaN dan +-Inf diganti 0 agar tidak meracuni state filter.
// len(src) harus kelipatan ukuran sampel.
func AppendDecoded(dst []float32, src []byte, enc Encoding) ([]float32, error) {
	bps := enc.BytesPerSample()
	if bps == 0 {
		return dst, fmt.Errorf("pcm: encoding tidak dikenal: %v", enc)
	}
	if len(src)%bps != 0 {
		return dst, fmt.Errorf("pcm: panjang %d byte bukan kelipatan %d (%v)", len(src), bps, enc)
	}
	dst = slices.Grow(dst, len(src)/bps)
	le := binary.LittleEndian
	switch enc {
	case EncodingInt16:
		for i := 0; i < len(src); i += 2 {
			dst = append(dst, float32(int16(le.Uint16(src[i:])))/32768)
		}
	case EncodingInt24:
		for i := 0; i < len(src); i += 3 {
			// byte tertinggi dibaca sebagai int8 agar tanda terbawa (sign extension).
			v := int32(src[i]) | int32(src[i+1])<<8 | int32(int8(src[i+2]))<<16
			dst = append(dst, float32(v)/8388608)
		}
	case EncodingInt32:
		for i := 0; i < len(src); i += 4 {
			dst = append(dst, float32(float64(int32(le.Uint32(src[i:])))/2147483648))
		}
	case EncodingFloat32:
		for i := 0; i < len(src); i += 4 {
			v := math.Float32frombits(le.Uint32(src[i:]))
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				v = 0
			}
			dst = append(dst, v)
		}
	}
	return dst, nil
}

// AppendInt16 mengubah float32 [-1, 1] menjadi int16 (dibulatkan, dijepit ke
// [-32768, 32767]) dan menambahkannya ke dst. clipped adalah jumlah sampel
// yang melampaui rentang dan dijepit. NaN menjadi 0 dan tidak dihitung clipped.
func AppendInt16(dst []int16, src []float32) (out []int16, clipped int) {
	dst = slices.Grow(dst, len(src))
	for _, v := range src {
		if math.IsNaN(float64(v)) {
			dst = append(dst, 0)
			continue
		}
		s := math.Floor(float64(v)*32768 + 0.5)
		switch {
		case s > 32767:
			s = 32767
			clipped++
		case s < -32768:
			s = -32768
			clipped++
		}
		dst = append(dst, int16(s))
	}
	return dst, clipped
}

// AppendInt16LE menambahkan sampel int16 sebagai byte little-endian ke dst
// (bentuk kawat untuk frame yang dikirim ke server).
func AppendInt16LE(dst []byte, src []int16) []byte {
	dst = slices.Grow(dst, 2*len(src))
	for _, v := range src {
		dst = binary.LittleEndian.AppendUint16(dst, uint16(v))
	}
	return dst
}
