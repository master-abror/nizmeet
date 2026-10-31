package pcm

import (
	"fmt"
	"math"
	"time"
)

// Batas desain resampler.
const (
	maxPhases = 1024    // L maksimum (pembilang rasio setelah disederhanakan)
	maxTaps   = 1 << 20 // panjang prototipe maksimum

	stopbandAttenDB = 70.0   // redaman stopband target (jendela Kaiser)
	transitionHz    = 1000.0 // lebar transisi, dibatasi 12,5% dari laju terendah
)

// Resampler adalah resampler streaming rasio rasional (polifase FIR, jendela
// Kaiser, fase linear) dengan state antar-potongan. Keluaran identik (bit demi
// bit) berapa pun ukuran potongan masukan.
//
// Cara kerja: y[k] = sum_j h[k*M - j*L] * x[j], dengan L/M = outRate/inRate
// setelah disederhanakan. Hanya keluaran yang dibutuhkan yang dihitung.
// Filter menambah latensi tetap (lihat Latency).
//
// Masukan harus berhingga (tanpa NaN/Inf); AppendDecoded sudah menjamin itu.
// Tipe ini tidak aman dipakai dari beberapa goroutine sekaligus.
type Resampler struct {
	l, m        int
	k           int         // panjang tap per fase
	taps        [][]float64 // [fase][k]
	buf         []float32   // riwayat + masukan yang belum habis dipakai
	base        int64       // indeks masukan absolut dari buf[0]
	next        int64       // indeks keluaran berikutnya
	passthrough bool
	latency     time.Duration
}

// NewResampler membuat resampler inRate -> outRate (Hz). Laju sama = passthrough.
func NewResampler(inRate, outRate int) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 {
		return nil, fmt.Errorf("pcm: laju tidak valid: %d -> %d", inRate, outRate)
	}
	g := gcd(inRate, outRate)
	l, m := outRate/g, inRate/g
	r := &Resampler{l: l, m: m}
	if l == 1 && m == 1 {
		r.passthrough = true
		return r, nil
	}
	if l > maxPhases {
		return nil, fmt.Errorf("pcm: rasio %d -> %d tidak didukung (L=%d > %d)", inRate, outRate, l, maxPhases)
	}
	h := designPrototype(inRate, outRate, l)
	if h == nil {
		return nil, fmt.Errorf("pcm: prototipe filter %d -> %d melebihi %d tap", inRate, outRate, maxTaps)
	}
	r.k = (len(h) + l - 1) / l
	flat := make([]float64, l*r.k)
	r.taps = make([][]float64, l)
	for p := 0; p < l; p++ {
		r.taps[p] = flat[p*r.k : (p+1)*r.k]
		for i := 0; i < r.k; i++ {
			if idx := p + i*l; idx < len(h) {
				r.taps[p][i] = h[idx]
			}
		}
	}
	fsHi := float64(inRate) * float64(l)
	r.latency = time.Duration(float64(len(h)-1) / 2 / fsHi * float64(time.Second))
	r.Reset()
	return r, nil
}

// Ratio mengembalikan L dan M (laju keluaran/masukan = L/M).
func (r *Resampler) Ratio() (l, m int) { return r.l, r.m }

// Latency adalah tunda grup filter (waktu) antara masukan dan keluarannya.
func (r *Resampler) Latency() time.Duration { return r.latency }

// Reset mengosongkan state seolah aliran baru dimulai (riwayat = nol).
func (r *Resampler) Reset() {
	if r.passthrough {
		return
	}
	r.buf = make([]float32, r.k-1, r.k-1+4096)
	r.base = -int64(r.k - 1)
	r.next = 0
}

// Process menambahkan keluaran hasil resampling src ke dst. Keluaran yang
// belum bisa dihitung (butuh masukan berikutnya) ditahan di state.
func (r *Resampler) Process(dst, src []float32) []float32 {
	if r.passthrough {
		return append(dst, src...)
	}
	r.buf = append(r.buf, src...)
	l, m := int64(r.l), int64(r.m)
	for {
		c := r.next * m
		phase := int(c % l)
		rel := c/l - r.base
		if rel >= int64(len(r.buf)) {
			break
		}
		ri := int(rel)
		var acc float64
		for i, tv := range r.taps[phase] {
			acc += tv * float64(r.buf[ri-i])
		}
		dst = append(dst, float32(acc))
		r.next++
	}
	// Buang riwayat yang tidak diperlukan lagi: keluaran berikutnya butuh
	// x[jNext-k+1 .. jNext].
	keepFrom := (r.next*m)/l - int64(r.k) + 1
	if drop := keepFrom - r.base; drop > 0 {
		if drop > int64(len(r.buf)) {
			drop = int64(len(r.buf))
		}
		n := copy(r.buf, r.buf[drop:])
		r.buf = r.buf[:n]
		r.base += drop
	}
	return dst
}

// designPrototype merancang low-pass prototipe pada laju inRate*l, sudah
// diskalakan dengan l agar gain DC tiap fase = 1. Tepi stopband = min(in,out)/2.
// Mengembalikan nil bila terlalu panjang.
func designPrototype(inRate, outRate, l int) []float64 {
	fsHi := float64(inRate) * float64(l)
	low := float64(min(inRate, outRate))
	stop := low / 2
	trans := math.Min(transitionHz, 0.125*low)
	cutoff := stop - trans/2 // pusat zona transisi
	dw := 2 * math.Pi * trans / fsHi
	n := int(math.Ceil((stopbandAttenDB-7.95)/(2.285*dw))) + 1
	if n%2 == 0 {
		n++
	}
	if n > maxTaps {
		return nil
	}
	beta := 0.1102 * (stopbandAttenDB - 8.7)
	i0beta := besselI0(beta)
	mid := float64(n-1) / 2
	wc := cutoff / fsHi // siklus per sampel
	h := make([]float64, n)
	for i := range h {
		x := float64(i) - mid
		var s float64
		if x == 0 {
			s = 2 * wc
		} else {
			s = math.Sin(2*math.Pi*wc*x) / (math.Pi * x)
		}
		rr := x / mid
		w := besselI0(beta*math.Sqrt(math.Max(0, 1-rr*rr))) / i0beta
		h[i] = s * w * float64(l)
	}
	return h
}

// besselI0 adalah fungsi Bessel termodifikasi orde nol (deret pangkat).
func besselI0(x float64) float64 {
	sum, term := 1.0, 1.0
	q := x * x / 4
	for k := 1; k < 200; k++ {
		term *= q / float64(k*k)
		sum += term
		if term < sum*1e-16 {
			break
		}
	}
	return sum
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
