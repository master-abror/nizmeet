package pcm

import "fmt"

// FrameSamples20ms16k adalah jumlah sampel satu frame 20 ms pada 16 kHz mono.
const FrameSamples20ms16k = 320

// Framer memotong aliran sampel mono int16 menjadi frame berukuran tetap.
// Sisa yang belum penuh ditahan sampai Push berikutnya. Tidak aman lintas goroutine.
type Framer struct {
	size int
	buf  []int16
}

// NewFramer membuat framer dengan ukuran frame (dalam sampel).
func NewFramer(size int) (*Framer, error) {
	if size < 1 {
		return nil, fmt.Errorf("pcm: ukuran frame %d tidak valid", size)
	}
	return &Framer{size: size, buf: make([]int16, 0, size)}, nil
}

// Size mengembalikan ukuran frame dalam sampel.
func (f *Framer) Size() int { return f.size }

// Pending mengembalikan jumlah sampel yang menunggu frame penuh.
func (f *Framer) Pending() int { return len(f.buf) }

// Reset membuang sampel yang tertahan.
func (f *Framer) Reset() { f.buf = f.buf[:0] }

// Push memasukkan sampel dan memanggil emit untuk setiap frame penuh, berurutan.
// Slice frame HANYA valid selama emit berjalan: salin bila perlu disimpan.
func (f *Framer) Push(samples []int16, emit func(frame []int16)) {
	if len(f.buf) > 0 {
		take := min(f.size-len(f.buf), len(samples))
		f.buf = append(f.buf, samples[:take]...)
		samples = samples[take:]
		if len(f.buf) < f.size {
			return
		}
		emit(f.buf)
		f.buf = f.buf[:0]
	}
	for len(samples) >= f.size {
		emit(samples[:f.size:f.size])
		samples = samples[f.size:]
	}
	f.buf = append(f.buf, samples...)
}
