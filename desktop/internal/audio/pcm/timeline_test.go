package pcm

import (
	"testing"
	"time"
)

func TestTimelineDeficit(t *testing.T) {
	tl, err := NewTimeline(16000)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1000, 0)
	tl.Anchor(t0)
	if d := tl.Deficit(t0.Add(time.Second)); d != 16000 {
		t.Fatalf("Deficit(+1s) = %d, want 16000", d)
	}
	tl.AddEmitted(16000)
	if d := tl.Deficit(t0.Add(time.Second)); d != 0 {
		t.Errorf("Deficit setelah diisi = %d, want 0", d)
	}
	if d := tl.Deficit(t0.Add(1500 * time.Millisecond)); d != 8000 {
		t.Errorf("Deficit(+1,5s) = %d, want 8000", d)
	}
	tl.AddEmitted(100)
	if d := tl.Deficit(t0.Add(1500 * time.Millisecond)); d != 7900 {
		t.Errorf("Deficit dengan kelebihan 100 = %d, want 7900", d)
	}
	if tl.Emitted() != 16100 {
		t.Errorf("Emitted = %d", tl.Emitted())
	}
}

func TestTimelineReanchorDropsDrift(t *testing.T) {
	tl, _ := NewTimeline(16000)
	t0 := time.Unix(1000, 0)
	tl.Anchor(t0)
	tl.AddEmitted(15900) // 1 detik berlalu tetapi baru 15900 sampel: selisih laju perangkat
	now := t0.Add(time.Second)
	tl.Anchor(now)
	if d := tl.Deficit(now.Add(time.Second)); d != 16000 {
		t.Errorf("Deficit setelah re-anchor = %d, want 16000 (selisih tidak boleh terbawa)", d)
	}
}

func TestTimelineAheadAndNoNegativeDeficit(t *testing.T) {
	tl, _ := NewTimeline(16000)
	t0 := time.Unix(1000, 0)
	tl.Anchor(t0)
	tl.AddEmitted(16800)
	now := t0.Add(time.Second)
	if d := tl.Deficit(now); d != 0 {
		t.Errorf("Deficit saat mendahului = %d, want 0", d)
	}
	if a := tl.Ahead(now); a != 50*time.Millisecond {
		t.Errorf("Ahead = %v, want 50ms", a)
	}
	if a := tl.Ahead(t0.Add(2 * time.Second)); a >= 0 {
		t.Errorf("Ahead saat tertinggal = %v, want negatif", a)
	}
}

func TestTimelineTargetExactAndMonotonic(t *testing.T) {
	tl, _ := NewTimeline(16000)
	t0 := time.Unix(1000, 0)
	tl.Anchor(t0)
	var prev int64
	for ms := 0; ms <= 3000; ms += 7 {
		got := tl.Target(t0.Add(time.Duration(ms) * time.Millisecond))
		if got < prev {
			t.Fatalf("Target mundur pada %d ms: %d < %d", ms, got, prev)
		}
		prev = got
		if ms%1000 == 0 && got != int64(ms)*16 {
			t.Errorf("Target(%d ms) = %d, want %d", ms, got, int64(ms)*16)
		}
	}
	if got := tl.Target(t0.Add(-time.Second)); got != 0 {
		t.Errorf("Target sebelum jangkar = %d, want 0", got)
	}
}

func TestNewTimelineError(t *testing.T) {
	if _, err := NewTimeline(0); err == nil {
		t.Error("ingin error untuk laju 0")
	}
}
