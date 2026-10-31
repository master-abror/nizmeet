package pcm

import "testing"

func TestDownmixStereoAverage(t *testing.T) {
	d, err := NewDownmix(2, 0x3) // FL|FR
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Apply(nil, []float32{1, 0, 0, 1, 0.5, 0.5})
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{0.5, 0.5, 0.5}, 1e-7)
}

func TestDownmix51IgnoresLFE(t *testing.T) {
	// FL|FR|FC|LFE|BL|BR; urutan kanal mengikuti urutan bit: LFE ada di indeks 3.
	d, err := NewDownmix(6, 0x3F)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Apply(nil, []float32{1, 1, 1, 100, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{1}, 1e-6)
}

func TestDownmixFallbackEqualWeights(t *testing.T) {
	cases := []struct {
		name     string
		channels int
		mask     uint32
		in       []float32
		want     float32
	}{
		{"mask nol", 2, 0, []float32{1, 0}, 0.5},
		{"mask tidak cocok jumlah kanal", 3, 0x3, []float32{3, 0, 0}, 1},
	}
	for _, c := range cases {
		d, err := NewDownmix(c.channels, c.mask)
		if err != nil {
			t.Fatal(err)
		}
		got, err := d.Apply(nil, c.in)
		if err != nil {
			t.Fatal(err)
		}
		assertF32(t, got, []float32{c.want}, 1e-6)
	}
}

func TestDownmixLFEExcludedWithTwoChannels(t *testing.T) {
	// Mask LFE|BL (0x8|0x10): kanal 0 = LFE (dibuang), kanal 1 = BL.
	d, err := NewDownmix(2, SpeakerLowFrequency|0x10)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Apply(nil, []float32{1, 0.5})
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{0.5}, 1e-7)
}

func TestDownmixMonoPassThrough(t *testing.T) {
	d, err := NewDownmix(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.Apply([]float32{7}, []float32{0.25, -0.5})
	if err != nil {
		t.Fatal(err)
	}
	assertF32(t, got, []float32{7, 0.25, -0.5}, 0)
}

func TestDownmixErrors(t *testing.T) {
	if _, err := NewDownmix(0, 0); err == nil {
		t.Error("ingin error untuk 0 kanal")
	}
	d, _ := NewDownmix(2, 0)
	if _, err := d.Apply(nil, make([]float32, 3)); err == nil {
		t.Error("ingin error untuk panjang bukan kelipatan kanal")
	}
}
