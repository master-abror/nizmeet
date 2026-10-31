package capture

import (
	"math"
	"testing"
	"time"
)

const qpc10ms = 100_000 // 10 ms dalam satuan 100 ns

func observeSeq(g *GapTracker, base time.Time, n int) {
	for i := 0; i < n; i++ {
		g.Observe(base.Add(time.Duration(i)*10*time.Millisecond), 480, 48000, 5_000_000+uint64(i)*qpc10ms, false)
	}
}

func TestGapTrackerContiguousStream(t *testing.T) {
	var g GapTracker
	observeSeq(&g, time.Unix(100, 0), 100)
	s := g.Snapshot()
	if s.Count != 0 || s.Max != 10*time.Millisecond {
		t.Errorf("Count=%d Max=%v", s.Count, s.Max)
	}
	if r := s.QPCRatio(); math.Abs(r-1) > 1e-9 {
		t.Errorf("QPCRatio = %v, want 1", r)
	}
	if s.Hist[0] != 99 { // 99 celah 10 ms
		t.Errorf("Hist[0] = %d, want 99", s.Hist[0])
	}
}

func TestGapTrackerClassifiesJitterSilenceOther(t *testing.T) {
	base := time.Unix(100, 0)
	var g GapTracker
	g.Observe(base, 480, 48000, 5_000_000, false)

	// Jitter: tiba 120 ms kemudian, tetapi jam capture kontinu (maju 10 ms).
	g.Observe(base.Add(120*time.Millisecond), 480, 48000, 5_000_000+qpc10ms, false)
	// Hening: tiba 1 s kemudian, jam capture ikut maju 1 s.
	g.Observe(base.Add(1120*time.Millisecond), 480, 48000, 5_000_000+qpc10ms+10_000_000, false)
	// Lain: tiba 200 ms kemudian, jam capture maju 100 ms (lubang 90 ms).
	g.Observe(base.Add(1320*time.Millisecond), 480, 48000, 5_000_000+qpc10ms+10_000_000+1_000_000, false)

	s := g.Snapshot()
	if s.Count != 3 || s.Jitter != 1 || s.Silence != 1 || s.Other != 1 || s.Unknown != 0 {
		t.Fatalf("klasifikasi: %+v", s)
	}
	if s.MaxJitter != 120*time.Millisecond {
		t.Errorf("MaxJitter = %v", s.MaxJitter)
	}
	if s.MaxSilenceHole != 990*time.Millisecond {
		t.Errorf("MaxSilenceHole = %v, want 990ms", s.MaxSilenceHole)
	}
	if s.Hist[4] != 2 || s.Hist[5] != 1 { // 120 ms dan 200 ms -> <=250ms; 1 s -> <=1s
		t.Errorf("Hist = %v", s.Hist)
	}
}

func TestGapTrackerUnknownWithoutQPC(t *testing.T) {
	base := time.Unix(100, 0)
	var g GapTracker
	g.Observe(base, 480, 48000, 0, false)
	g.Observe(base.Add(300*time.Millisecond), 480, 48000, 0, false)
	g.Observe(base.Add(700*time.Millisecond), 480, 48000, 9_000_000, true) // TIMESTAMP_ERROR
	s := g.Snapshot()
	if s.Count != 2 || s.Unknown != 2 || s.Jitter+s.Silence+s.Other != 0 {
		t.Errorf("%+v", s)
	}
	if s.QPCRatio() != 0 {
		t.Errorf("QPCRatio tanpa data = %v", s.QPCRatio())
	}
}

func TestGapTrackerResetAndIntervalMax(t *testing.T) {
	base := time.Unix(100, 0)
	var g GapTracker
	g.Observe(base, 480, 48000, 5_000_000, false)
	g.Observe(base.Add(80*time.Millisecond), 480, 48000, 5_000_000+8*qpc10ms, false)
	if m := g.TakeIntervalMax(); m != 80*time.Millisecond {
		t.Errorf("TakeIntervalMax = %v", m)
	}
	if m := g.TakeIntervalMax(); m != 0 {
		t.Errorf("TakeIntervalMax kedua = %v, want 0", m)
	}
	before := g.Snapshot()
	g.Reset()
	g.Observe(base.Add(time.Hour), 480, 48000, 7_000_000, false) // paket pertama stream baru
	after := g.Snapshot()
	if after.Count != before.Count || after.ArrivalSpan != before.ArrivalSpan || after.Max != before.Max {
		t.Errorf("Reset tidak boleh menghitung celah lintas stream: %+v vs %+v", before, after)
	}
}

func TestGapTrackerFlagsWrongQPCUnit(t *testing.T) {
	base := time.Unix(100, 0)
	var g GapTracker
	// Seolah satuan QPC 1 us (bukan 100 ns): selisih 10 ms = 10_000 satuan.
	for i := 0; i < 50; i++ {
		g.Observe(base.Add(time.Duration(i)*10*time.Millisecond), 480, 48000, 5_000_000+uint64(i)*10_000, false)
	}
	if r := g.Snapshot().QPCRatio(); r > 0.2 {
		t.Errorf("QPCRatio = %v, harus jauh dari 1 bila satuan salah", r)
	}
}
