package pcm

import (
	"fmt"
	"math/bits"
	"slices"
)

// SpeakerLowFrequency adalah bit kanal LFE pada channel mask.
const SpeakerLowFrequency = 0x8 // SPEAKER_LOW_FREQUENCY | mmreg.h:2048

// Downmix menggabungkan sampel interleaved N kanal menjadi mono.
//
// Bobot: rata-rata sama rata di semua kanal, kecuali kanal LFE (bobot 0) bila
// channel mask diketahui dan jumlah bit-nya sama dengan jumlah kanal. Urutan
// kanal pada aliran interleaved dianggap mengikuti urutan bit mask dari yang
// terendah (aturan dari dokumentasi WAVEFORMATEXTENSIBLE, BUKAN dari header
// lampiran: TIDAK TERVERIFIKASI). Bila mask kosong atau tidak cocok, semua
// kanal berbobot sama.
//
// Rata-rata dipilih agar hasil tidak pernah melebihi rentang masukan (tanpa
// clipping). Konsekuensinya konten yang hanya ada di satu kanal turun 6 dB pada
// stereo; kalibrasi ambang gate VAD harus memperhitungkan itu.
type Downmix struct {
	channels int
	weights  []float32
}

// NewDownmix membuat downmixer untuk jumlah kanal dan channel mask tertentu (mask 0 = tidak diketahui).
func NewDownmix(channels int, mask uint32) (*Downmix, error) {
	if channels < 1 {
		return nil, fmt.Errorf("pcm: jumlah kanal %d tidak valid", channels)
	}
	w := make([]float32, channels)
	for i := range w {
		w[i] = 1
	}
	if channels > 1 && mask != 0 && bits.OnesCount32(mask) == channels {
		idx := 0
		for bit := 0; bit < 32; bit++ {
			m := uint32(1) << bit
			if mask&m == 0 {
				continue
			}
			if m == SpeakerLowFrequency {
				w[idx] = 0
			}
			idx++
		}
	}
	var sum float32
	for _, v := range w {
		sum += v
	}
	if sum == 0 { // semua kanal LFE: jangan membisukan, kembali ke bobot sama
		for i := range w {
			w[i] = 1
		}
		sum = float32(channels)
	}
	for i := range w {
		w[i] /= sum
	}
	return &Downmix{channels: channels, weights: w}, nil
}

// Channels mengembalikan jumlah kanal masukan.
func (d *Downmix) Channels() int { return d.channels }

// Apply menambahkan hasil mono dari src (interleaved, len kelipatan jumlah kanal) ke dst.
func (d *Downmix) Apply(dst, src []float32) ([]float32, error) {
	if len(src)%d.channels != 0 {
		return dst, fmt.Errorf("pcm: panjang %d bukan kelipatan %d kanal", len(src), d.channels)
	}
	if d.channels == 1 {
		return append(dst, src...), nil
	}
	n := len(src) / d.channels
	dst = slices.Grow(dst, n)
	for i := 0; i < n; i++ {
		frame := src[i*d.channels : (i+1)*d.channels]
		var acc float32
		for c, w := range d.weights {
			acc += w * frame[c]
		}
		dst = append(dst, acc)
	}
	return dst, nil
}
