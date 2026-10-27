package pcm

import (
	"math"
	"testing"
)

// BenchmarkChainFloat32Stereo48k mengukur biaya CPU rantai
// decode -> downmix -> resample 48k->16k -> int16 untuk 1 detik audio
// (100 paket @ 10 ms), memakai ulang buffer seperti capturer nanti.
// pct_core = persen satu core yang dipakai untuk audio real-time.
//
//	go test -run '^$' -bench Chain ./internal/audio/pcm
func BenchmarkChainFloat32Stereo48k(b *testing.B) {
	const rate = 48000
	raw := make([]byte, 0, rate*2*4)
	for i := 0; i < rate; i++ {
		v := float32(0.3 * math.Sin(2*math.Pi*440*float64(i)/rate))
		raw = append(raw, f32bytes(v, v)...)
	}
	dm, err := NewDownmix(2, 0x3)
	if err != nil {
		b.Fatal(err)
	}
	rs, err := NewResampler(rate, 16000)
	if err != nil {
		b.Fatal(err)
	}
	var f32, mono, out []float32
	var i16 []int16
	const chunk = 480 * 2 * 4 // 10 ms stereo float32
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for off := 0; off+chunk <= len(raw); off += chunk {
			f32, _ = AppendDecoded(f32[:0], raw[off:off+chunk], EncodingFloat32)
			mono, _ = dm.Apply(mono[:0], f32)
			out = rs.Process(out[:0], mono)
			i16, _ = AppendInt16(i16[:0], out)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/1e9*100, "pct_core")
}
