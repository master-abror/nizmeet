package capture

import (
	"testing"
	"time"
)

const qpcBase = 5_000_000

// qpcFor memberi QPCPosition (satuan 100 ns) untuk frame ke-n pada 48 kHz.
func qpcFor(frame int) uint64 { return qpcBase + uint64(frame)*10_000_000/48000 }

// feedQPC seperti feedSteady tetapi mengisi QPCPosition (jam capture kontinu).
func feedQPC(t *testing.T, p *Pipeline, clk *fakeClock, n, startFrame int) int {
	t.Helper()
	for i := 0; i < n; i++ {
		clk.Advance(10 * time.Millisecond)
		pk := sinePacket(48000, 480, startFrame, uint64(startFrame))
		pk.QPCPosition = qpcFor(startFrame)
		if err := p.HandlePacket(pk, clk.Now()); err != nil {
			t.Fatal(err)
		}
		startFrame += 480
	}
	return startFrame
}

func qpcPacket(next int, extraQPC uint64) Packet {
	pk := sinePacket(48000, 480, next, uint64(next))
	pk.QPCPosition = qpcFor(next) + extraQPC
	return pk
}

// Jitter (macet atau terlambat) selama 400 ms dengan jam capture kontinu: tidak boleh ada pad.
func TestCatchUpSkipsJitterWhenQPCContinuous(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedQPC(t, p, clk, 10, 0)
	anchor := p.Emitted()
	clk.Advance(400 * time.Millisecond)
	if err := p.HandlePacket(qpcPacket(next, 0), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if p.SilenceGap() != 0 {
		t.Errorf("SilenceGap = %d, want 0 untuk jitter", p.SilenceGap())
	}
	if got := p.Emitted() - anchor; got != 160 {
		t.Errorf("selisih emitted = %d, want 160", got)
	}
	if s := p.Gaps(); s.Jitter != 1 {
		t.Errorf("Gaps: %+v", s)
	}
}

// Hening 400 ms (lubang jam capture): pad sepanjang lubang itu, persis.
func TestCatchUpFillsExactQPCHole(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedQPC(t, p, clk, 10, 0)
	anchor := p.Emitted()
	clk.Advance(410 * time.Millisecond)
	if err := p.HandlePacket(qpcPacket(next, 4_000_000), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if g := p.SilenceGap(); g != 6400 { // 400 ms x 16 kHz
		t.Errorf("SilenceGap = %d, want 6400", g)
	}
	if got := p.Emitted() - anchor; got != 6560 { // 410 ms: 6400 sunyi + 160 audio
		t.Errorf("selisih emitted = %d, want 6560", got)
	}
}

// Hening 100 ms (di bawah ambang waktu lama 250 ms): sekarang tetap diisi, tidak dimampatkan.
func TestCatchUpFillsShortHoleBelowOldThreshold(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedQPC(t, p, clk, 10, 0)
	clk.Advance(110 * time.Millisecond)
	if err := p.HandlePacket(qpcPacket(next, 1_000_000), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if g := p.SilenceGap(); g != 1600 {
		t.Errorf("SilenceGap = %d, want 1600", g)
	}
}

// Hening yang sudah diisi Tick tidak boleh diisi dua kali saat paket tiba.
func TestCatchUpNoDoubleFillAfterTick(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedQPC(t, p, clk, 10, 0)
	for i := 0; i < 100; i++ { // 2 detik hening, Tick mengisi
		clk.Advance(20 * time.Millisecond)
		p.Tick(clk.Now())
	}
	clk.Advance(10 * time.Millisecond)
	if err := p.HandlePacket(qpcPacket(next, 20_000_000), clk.Now()); err != nil { // lubang 2 s
		t.Fatal(err)
	}
	if p.SilenceIdle() == 0 {
		t.Error("Tick harus mengisi hening")
	}
	if p.SilenceGap() != 0 {
		t.Errorf("SilenceGap = %d, want 0 (sudah diisi Tick)", p.SilenceGap())
	}
}

// Setelah buka ulang pengamat celah di-reset: paket pertama stream baru tidak
// terklasifikasi, jadi celah diisi dengan aturan jam dinding walau QPC ada.
func TestCatchUpAfterReopenFallsBackToWallClock(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedQPC(t, p, clk, 10, 0)
	clk.Advance(600 * time.Millisecond) // Open memblokir
	if err := p.SetFormat(f32Format(48000)); err != nil {
		t.Fatal(err)
	}
	clk.Advance(10 * time.Millisecond)
	if err := p.HandlePacket(qpcPacket(next, 6_000_000), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if g := p.SilenceGap(); g < 9500 || g > 9560 { // 610 ms x 16 kHz - ekor 70 - paket 160 = 9530
		t.Errorf("SilenceGap = %d, want ~9530", g)
	}
}
