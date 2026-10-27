package pcm

import "testing"

func collect(f *Framer, chunks ...[]int16) [][]int16 {
	var frames [][]int16
	emit := func(fr []int16) { frames = append(frames, append([]int16(nil), fr...)) }
	for _, c := range chunks {
		f.Push(c, emit)
	}
	return frames
}

func ramp(start, n int) []int16 {
	x := make([]int16, n)
	for i := range x {
		x[i] = int16(start + i)
	}
	return x
}

func checkRamp(t *testing.T, frames [][]int16, size, wantFrames int) {
	t.Helper()
	if len(frames) != wantFrames {
		t.Fatalf("jumlah frame %d, want %d", len(frames), wantFrames)
	}
	for n, fr := range frames {
		if len(fr) != size {
			t.Fatalf("frame %d berukuran %d, want %d", n, len(fr), size)
		}
		for i, v := range fr {
			if v != int16(n*size+i) {
				t.Fatalf("frame %d[%d] = %d, want %d", n, i, v, n*size+i)
			}
		}
	}
}

func TestFramerContiguousFrames(t *testing.T) {
	f, err := NewFramer(FrameSamples20ms16k)
	if err != nil {
		t.Fatal(err)
	}
	sizes := []int{1, 7, 100, 319, 320, 253} // jumlah 1000
	var chunks [][]int16
	pos := 0
	for _, s := range sizes {
		chunks = append(chunks, ramp(pos, s))
		pos += s
	}
	frames := collect(f, chunks...)
	checkRamp(t, frames, 320, 3)
	if f.Pending() != 40 {
		t.Errorf("Pending = %d, want 40", f.Pending())
	}
}

func TestFramerChunkInvariance(t *testing.T) {
	all := ramp(0, 1000)
	f1, _ := NewFramer(320)
	checkRamp(t, collect(f1, all), 320, 3)

	f2, _ := NewFramer(320)
	var singles [][]int16
	for i := range all {
		singles = append(singles, all[i:i+1])
	}
	checkRamp(t, collect(f2, singles...), 320, 3)
	if f1.Pending() != f2.Pending() {
		t.Errorf("Pending beda: %d vs %d", f1.Pending(), f2.Pending())
	}
}

func TestFramerResetAndErrors(t *testing.T) {
	f, _ := NewFramer(320)
	collect(f, ramp(0, 100))
	if f.Pending() != 100 {
		t.Fatalf("Pending = %d", f.Pending())
	}
	f.Reset()
	if f.Pending() != 0 {
		t.Errorf("Pending setelah Reset = %d", f.Pending())
	}
	if _, err := NewFramer(0); err == nil {
		t.Error("ingin error untuk ukuran 0")
	}
}
