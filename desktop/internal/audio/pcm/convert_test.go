package pcm

import (
	"encoding/binary"
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func assertF32(t *testing.T, got, want []float32, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("panjang %d, want %d (got=%v)", len(got), len(want), got)
	}
	for i := range got {
		if !near(float64(got[i]), float64(want[i]), tol) {
			t.Errorf("[%d] got %v, want %v", i, got[i], want[i])
		}
	}
}

func f32bytes(vals ...float32) []byte {
	b := make([]byte, 0, 4*len(vals))
	for _, v := range vals {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
	}
	return b
}

func TestAppendDecodedInt16(t *testing.T) {
	src := []byte{0x00, 0x80, 0xFF, 0x7F, 0x00, 0x00}
	got, err := AppendDecoded(nil, src, EncodingInt16)
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{-1, 32767.0 / 32768.0, 0}, 0)
}

func TestAppendDecodedInt24(t *testing.T) {
	src := []byte{
		0x00, 0x00, 0x80, // -8388608
		0xFF, 0xFF, 0x7F, // 8388607
		0x00, 0x00, 0x00, // 0
		0xFF, 0xFF, 0xFF, // -1
	}
	got, err := AppendDecoded(nil, src, EncodingInt24)
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{-1, 8388607.0 / 8388608.0, 0, -1.0 / 8388608.0}, 1e-9)
}

func TestAppendDecodedInt32(t *testing.T) {
	src := []byte{
		0x00, 0x00, 0x00, 0x80, // -2147483648
		0x00, 0x00, 0x00, 0x40, // 1073741824
		0x00, 0x00, 0x00, 0x00,
	}
	got, err := AppendDecoded(nil, src, EncodingInt32)
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{-1, 0.5, 0}, 1e-9)
}

func TestAppendDecodedFloat32SanitizesNonFinite(t *testing.T) {
	nan := math.Float32frombits(0x7fc00000)
	src := f32bytes(0.5, nan, float32(math.Inf(1)), float32(math.Inf(-1)), -0.25)
	got, err := AppendDecoded(nil, src, EncodingFloat32)
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{0.5, 0, 0, 0, -0.25}, 0)
}

func TestAppendDecodedAppendsToDst(t *testing.T) {
	dst := []float32{9}
	got, err := AppendDecoded(dst, f32bytes(0.5), EncodingFloat32)
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{9, 0.5}, 0)
}

func TestAppendDecodedErrors(t *testing.T) {
	if _, err := AppendDecoded(nil, make([]byte, 3), EncodingInt16); err == nil {
		t.Error("ingin error untuk panjang bukan kelipatan")
	}
	if _, err := AppendDecoded(nil, make([]byte, 4), EncodingUnknown); err == nil {
		t.Error("ingin error untuk encoding tidak dikenal")
	}
}

func TestAppendInt16(t *testing.T) {
	nan := float32(math.NaN())
	src := []float32{0, 0.5, -1, 1, 2, -2, nan, 1.0 / 32768.0}
	got, clipped := AppendInt16(nil, src)
	want := []int16{0, 16384, -32768, 32767, 32767, -32768, 0, 1}
	if len(got) != len(want) {
		t.Fatalf("panjang %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %d, want %d", i, got[i], want[i])
		}
	}
	// 1.0 (=> 32768), 2.0, dan -2.0 melampaui rentang; -1.0 tepat -32768 (tidak clipped).
	if clipped != 3 {
		t.Errorf("clipped = %d, want 3", clipped)
	}
}

func TestAppendInt16LE(t *testing.T) {
	got := AppendInt16LE(nil, []int16{1, -2, 0x1234})
	want := []byte{0x01, 0x00, 0xFE, 0xFF, 0x34, 0x12}
	if string(got) != string(want) {
		t.Errorf("got % X, want % X", got, want)
	}
}

func TestDecodeThenInt16RoundTrip(t *testing.T) {
	src := make([]byte, 0, 2*5)
	in := []int16{-32768, -1, 0, 1, 32767}
	for _, v := range in {
		src = binary.LittleEndian.AppendUint16(src, uint16(v))
	}
	f, err := AppendDecoded(nil, src, EncodingInt16)
	if err != nil {
		t.Fatal(err)
	}
	out, clipped := AppendInt16(nil, f)
	if clipped != 0 {
		t.Errorf("clipped = %d", clipped)
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("[%d] got %d, want %d", i, out[i], in[i])
		}
	}
}
