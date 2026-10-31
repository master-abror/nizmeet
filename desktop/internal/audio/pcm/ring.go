package pcm

import (
	"fmt"
	"sync/atomic"
)

// Ring adalah ring buffer int16 single-producer/single-consumer tanpa kunci.
// Tepat SATU goroutine boleh memanggil Write/Free dan tepat SATU goroutine lain
// memanggil Read. Len, Cap, dan Dropped aman dari mana saja.
//
// Kebijakan overrun: bila penuh, Write membuang sampel BARU yang tidak muat
// (yang sudah di buffer dipertahankan) dan menghitungnya di Dropped. Itu
// konsekuensi desain tanpa kunci: hanya konsumen yang boleh memajukan head.
//
// Kebenaran bergantung pada atomic berurutan konsisten milik Go (tulis data lalu
// Store tail; Load tail lalu baca data), bukan pada race detector.
type Ring struct {
	buf     []int16
	head    atomic.Uint64 // jumlah sampel yang sudah dibaca (ditulis konsumen)
	tail    atomic.Uint64 // jumlah sampel yang sudah ditulis (ditulis produsen)
	dropped atomic.Uint64
}

// NewRing membuat ring buffer berkapasitas capacity sampel.
func NewRing(capacity int) (*Ring, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("pcm: kapasitas ring %d tidak valid", capacity)
	}
	return &Ring{buf: make([]int16, capacity)}, nil
}

// Cap mengembalikan kapasitas dalam sampel.
func (r *Ring) Cap() int { return len(r.buf) }

// Len mengembalikan jumlah sampel yang siap dibaca (perkiraan bila dipanggil di luar produsen/konsumen).
func (r *Ring) Len() int {
	h := r.head.Load()
	return int(r.tail.Load() - h)
}

// Free mengembalikan ruang kosong; hanya boleh diandalkan oleh produsen
// (konsumen hanya bisa menambah ruang kosong, jadi hasilnya konservatif).
func (r *Ring) Free() int { return len(r.buf) - r.Len() }

// Dropped mengembalikan total sampel yang dibuang karena buffer penuh.
func (r *Ring) Dropped() uint64 { return r.dropped.Load() }

// Write menulis sebanyak yang muat dan mengembalikan jumlah tertulis;
// sisanya dibuang dan dihitung di Dropped. Hanya untuk goroutine produsen.
func (r *Ring) Write(p []int16) int {
	t := r.tail.Load()
	free := len(r.buf) - int(t-r.head.Load())
	n := min(len(p), free)
	if n < len(p) {
		r.dropped.Add(uint64(len(p) - n))
	}
	if n == 0 {
		return 0
	}
	idx := int(t % uint64(len(r.buf)))
	c := copy(r.buf[idx:], p[:n])
	copy(r.buf, p[c:n])
	r.tail.Store(t + uint64(n))
	return n
}

// Read mengisi p dengan sampel yang tersedia dan mengembalikan jumlahnya.
// Hanya untuk goroutine konsumen.
func (r *Ring) Read(p []int16) int {
	h := r.head.Load()
	avail := int(r.tail.Load() - h)
	n := min(len(p), avail)
	if n == 0 {
		return 0
	}
	idx := int(h % uint64(len(r.buf)))
	c := copy(p[:n], r.buf[idx:])
	copy(p[c:n], r.buf)
	r.head.Store(h + uint64(n))
	return n
}
