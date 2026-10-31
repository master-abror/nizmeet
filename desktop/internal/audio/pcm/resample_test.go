package pcm

import (
	"math"
	"testing"
)

func sine(freq float64, rate, n int, amp float64) []float32 {
	x := make([]float32, n)
	for i := range x {
		x[i] = float32(amp * math.Sin(2*math.Pi*freq*float64(i)/float64(rate)))
	}
	return x
}

func rms(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v) * float64(v)
	}
	return math.Sqrt(s / float64(len(x)))
}

// noise menghasilkan derau deterministik (LCG) pada [-0,5; 0,5).
func noise(n int, seed uint32) []float32 {
	x := make([]float32, n)
	s := seed
	for i := range x {
		s = s*1664525 + 1013904223
		x[i] = float32(s>>8)/float32(1<<24) - 0.5
	}
	return x
}

func process(t *testing.T, in, out int, x []float32, chunk int) []float32 {
	t.Helper()
	r, err := NewResampler(in, out)
	if err != nil {
		t.Fatal(err)
	}
	var y []float32
	for off := 0; off < len(x); off += chunk {
		end := min(off+chunk, len(x))
		y = r.Process(y, x[off:end])
	}
	return y
}

func TestResamplerOutputCount(t *testing.T) {
	for _, in := range []int{48000, 44100, 96000, 24000, 8000, 16000} {
		r, err := NewResampler(in, 16000)
		if err != nil {
			t.Fatal(err)
		}
		l, m := r.Ratio()
		x := noise(in, 1) // tepat 1 detik masukan
		y := r.Process(nil, x)
		want := (len(x)*l + m - 1) / m
		if len(y) != want {
			t.Errorf("%d->16000: keluaran %d, want %d (L=%d M=%d)", in, len(y), want, l, m)
		}
		if in != 16000 && len(y) != 16000 {
			t.Errorf("%d->16000: 1 s masukan harus menjadi 16000 sampel, dapat %d", in, len(y))
		}
	}
}

func TestResamplerChunkInvariance(t *testing.T) {
	for _, in := range []int{48000, 44100} {
		x := noise(6000, 7)
		ref := process(t, in, 16000, x, len(x))
		for _, chunk := range []int{1, 7, 480, 1000, 4096} {
			got := process(t, in, 16000, x, chunk)
			if len(got) != len(ref) {
				t.Fatalf("%d chunk=%d: panjang %d, ref %d", in, chunk, len(got), len(ref))
			}
			for i := range ref {
				if got[i] != ref[i] {
					t.Fatalf("%d chunk=%d: beda di [%d]: %v vs %v", in, chunk, i, got[i], ref[i])
				}
			}
		}
	}
}

func TestResamplerResetRestartsStream(t *testing.T) {
	x := noise(3000, 3)
	r, err := NewResampler(48000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	first := r.Process(nil, x)
	r.Reset()
	second := r.Process(nil, x)
	if len(first) != len(second) {
		t.Fatalf("panjang beda: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("beda di [%d]", i)
		}
	}
}

func TestResamplerPassbandGain(t *testing.T) {
	for _, in := range []int{48000, 44100, 96000} {
		x := sine(1000, in, in, 0.5)
		y := process(t, in, 16000, x, 480)
		got := rms(y[800:]) / (0.5 / math.Sqrt2) // lewati 50 ms transien awal
		if !near(got, 1, 0.01) {
			t.Errorf("%d->16000: gain 1 kHz = %.5f, want 1 +-1%%", in, got)
		}
	}
}

// Tanpa low-pass, nada di atas 8 kHz akan terlipat (aliasing) ke dalam pita.
func TestResamplerStopbandAttenuation(t *testing.T) {
	cases := []struct {
		in   int
		freq float64
	}{{48000, 10000}, {44100, 10000}, {96000, 20000}, {24000, 10000}}
	for _, c := range cases {
		x := sine(c.freq, c.in, c.in, 0.5)
		y := process(t, c.in, 16000, x, 480)
		ratio := rms(y[800:]) / (0.5 / math.Sqrt2)
		if ratio > 1e-3 { // lebih buruk dari -60 dB
			t.Errorf("%d Hz @ %d: sisa %.2e (%.1f dB), want < -60 dB", int(c.freq), c.in, ratio, 20*math.Log10(ratio))
		}
	}
}

// naive menghitung definisi langsung y[k] = sum_j h[k*M - j*L] * x[j].
func naive(h []float64, l, m int, x []float32) []float64 {
	y := make([]float64, (len(x)*l+m-1)/m)
	for k := range y {
		c := k * m
		var acc float64
		for j := c / l; j >= 0 && c-j*l < len(h); j-- {
			if j < len(x) {
				acc += h[c-j*l] * float64(x[j])
			}
		}
		y[k] = acc
	}
	return y
}

func TestResamplerMatchesDefinition(t *testing.T) {
	for _, in := range []int{44100, 48000, 24000} {
		r, err := NewResampler(in, 16000)
		if err != nil {
			t.Fatal(err)
		}
		l, m := r.Ratio()
		h := designPrototype(in, 16000, l)
		x := noise(3000, 11)
		got := process(t, in, 16000, x, 333)
		want := naive(h, l, m, x)
		if len(got) != len(want) {
			t.Fatalf("%d: panjang %d vs %d", in, len(got), len(want))
		}
		for i := range got {
			if !near(float64(got[i]), want[i], 1e-5) {
				t.Fatalf("%d: beda di [%d]: %v vs %v", in, i, got[i], want[i])
			}
		}
	}
}

func TestResamplerLatencyMatchesImpulsePeak(t *testing.T) {
	for _, in := range []int{48000, 44100} {
		x := make([]float32, 2000)
		x[0] = 1
		y := process(t, in, 16000, x, 500)
		peak := 0
		for i, v := range y {
			if math.Abs(float64(v)) > math.Abs(float64(y[peak])) {
				peak = i
			}
		}
		r, _ := NewResampler(in, 16000)
		want := r.Latency().Seconds() * 16000
		if math.Abs(float64(peak)-want) > 1.5 {
			t.Errorf("%d: puncak impuls di %d, latensi %.2f sampel", in, peak, want)
		}
	}
}

func TestResamplerPassthrough(t *testing.T) {
	r, err := NewResampler(16000, 16000)
	if err != nil {
		t.Fatal(err)
	}
	x := noise(100, 5)
	y := r.Process([]float32{9}, x)
	if len(y) != 101 || y[0] != 9 || y[1] != x[0] || y[100] != x[99] {
		t.Errorf("passthrough salah: len=%d", len(y))
	}
}

func TestNewResamplerErrors(t *testing.T) {
	if _, err := NewResampler(0, 16000); err == nil {
		t.Error("ingin error untuk laju 0")
	}
	if _, err := NewResampler(44101, 16000); err == nil { // gcd=1 -> L=16000 > batas
		t.Error("ingin error untuk rasio yang L-nya melebihi batas")
	}
}
