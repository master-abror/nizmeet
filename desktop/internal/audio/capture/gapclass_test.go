package capture

import (
	"testing"
	"time"
)

func TestObserveReturnsClassAndHole(t *testing.T) {
	base := time.Unix(100, 0)
	var g GapTracker
	if c, h := g.Observe(base, 480, 48000, 5_000_000, false); c != GapNone || h != 0 {
		t.Fatalf("paket pertama: %v %v, want GapNone 0", c, h)
	}
	// Jitter: tiba 120 ms kemudian, jam capture kontinu.
	if c, h := g.Observe(base.Add(120*time.Millisecond), 480, 48000, 5_000_000+qpc10ms, false); c != GapJitter || h != 0 {
		t.Errorf("jitter: %v %v", c, h)
	}
	// Hening: tiba 1 s kemudian, jam capture ikut maju 1 s (lubang 990 ms).
	if c, h := g.Observe(base.Add(1120*time.Millisecond), 480, 48000, 5_000_000+qpc10ms+10_000_000, false); c != GapSilence || h != 990*time.Millisecond {
		t.Errorf("hening: %v %v, want GapSilence 990ms", c, h)
	}
	// Lain: tiba 200 ms kemudian, jam capture maju 100 ms (lubang 90 ms).
	if c, _ := g.Observe(base.Add(1320*time.Millisecond), 480, 48000, 5_000_000+qpc10ms+10_000_000+1_000_000, false); c != GapOther {
		t.Errorf("lain: %v", c)
	}
	// Tanpa QPC.
	if c, _ := g.Observe(base.Add(1620*time.Millisecond), 480, 48000, 0, false); c != GapUnknown {
		t.Errorf("tanpa QPC: %v", c)
	}
	// Celah kecil (10 ms) tidak diklasifikasi.
	if c, h := g.Observe(base.Add(1630*time.Millisecond), 480, 48000, 0, false); c != GapNone || h != 0 {
		t.Errorf("celah kecil: %v %v", c, h)
	}
}
