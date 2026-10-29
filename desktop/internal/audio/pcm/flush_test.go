package pcm

import (
	"math"
	"testing"
)

func maxAbs(x []float32) float64 {
	var m float64
	for _, v := range x {
		m = math.Max(m, math.Abs(float64(v)))
	}
	return m
}

// Impuls di sampel terakhir: tanpa Flush puncaknya masih tertahan di filter
// (latensi ~2 ms); setelah Flush puncaknya keluar.
func TestResamplerFlushEmitsTail(t *testing.T) {
	for _, in := range []int{48000, 44100} {
		r, err := NewResampler(in, 16000)
		if err != nil {
			t.Fatal(err)
		}
		x := make([]float32, 100)
		x[99] = 1
		y := r.Process(nil, x)
		if before := maxAbs(y); before > 0.01 {
			t.Fatalf("%d: sebelum Flush sudah ada puncak %.4f", in, before)
		}
		n := len(y)
		y = r.Flush(y)
		if len(y) <= n {
			t.Fatalf("%d: Flush tidak mengeluarkan sampel", in)
		}
		if after := maxAbs(y); after < 0.15 {
			t.Errorf("%d: setelah Flush puncak hanya %.4f, ekor hilang", in, after)
		}
	}
}

func TestResamplerFlushThenResetMatchesFresh(t *testing.T) {
	r, err := NewResampler(48000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	r.Process(nil, noise(3000, 21))
	r.Flush(nil)
	x := noise(2000, 22)
	got := r.Process(nil, x)
	want := process(t, 48000, 16000, x, len(x))
	if len(got) != len(want) {
		t.Fatalf("panjang %d vs %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("beda di [%d]: state lama bocor setelah Flush", i)
		}
	}
}

func TestResamplerFlushPassthrough(t *testing.T) {
	r, _ := NewResampler(16000, 16000)
	dst := []float32{1, 2}
	if got := r.Flush(dst); len(got) != 2 {
		t.Errorf("passthrough Flush mengubah dst: %v", got)
	}
}
