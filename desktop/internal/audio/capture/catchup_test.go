package capture

import (
	"context"
	"testing"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

func TestPipelineCatchUpFillsLongGap(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedSteady(t, p, clk, 48000, 10, 0)
	anchor := p.Emitted()
	clk.Advance(1300 * time.Millisecond) // celah tanpa Tick (mis. Open memblokir)
	if err := p.HandlePacket(sinePacket(48000, 480, next, uint64(next)), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if got := p.Emitted() - anchor; got != 20800 { // 1,3 s x 16000, tepat
		t.Errorf("selisih emitted = %d, want 20800", got)
	}
	if g := p.SilenceGap(); g != 20640 {
		t.Errorf("SilenceGap = %d, want 20640", g)
	}
	if p.SilenceIdle() != 0 || p.Resyncs() != 0 {
		t.Errorf("idle=%d resyncs=%d", p.SilenceIdle(), p.Resyncs())
	}
}

func TestPipelineCatchUpIgnoresJitter(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedSteady(t, p, clk, 48000, 10, 0)
	anchor := p.Emitted()
	clk.Advance(120 * time.Millisecond) // di bawah IdleThreshold: jitter, bukan celah
	if err := p.HandlePacket(sinePacket(48000, 480, next, uint64(next)), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if got := p.Emitted() - anchor; got != 160 {
		t.Errorf("selisih emitted = %d, want 160 (tanpa pad)", got)
	}
	if p.SilenceGap() != 0 {
		t.Errorf("SilenceGap = %d, want 0", p.SilenceGap())
	}
}

func TestPipelineNoDoublePadAfterIdleTicks(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedSteady(t, p, clk, 48000, 10, 0)
	for i := 0; i < 50; i++ {
		clk.Advance(20 * time.Millisecond)
		p.Tick(clk.Now())
	}
	clk.Advance(10 * time.Millisecond)
	if err := p.HandlePacket(sinePacket(48000, 480, next, uint64(next)), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if p.SilenceIdle() == 0 {
		t.Error("idle harus terisi oleh Tick")
	}
	if p.SilenceGap() != 0 {
		t.Errorf("SilenceGap = %d, want 0 (Tick sudah mengisi)", p.SilenceGap())
	}
}

func TestPipelineCatchUpCapped(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	next := feedSteady(t, p, clk, 48000, 10, 0)
	anchor := p.Emitted()
	clk.Advance(60 * time.Second)
	if err := p.HandlePacket(sinePacket(48000, 480, next, uint64(next)), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if g := p.SilenceGap(); g != 80000 { // MaxCatchUp 5 s
		t.Errorf("SilenceGap = %d, want 80000", g)
	}
	if p.Resyncs() != 1 {
		t.Errorf("Resyncs = %d, want 1", p.Resyncs())
	}
	if got := p.Emitted() - anchor; got != 80160 {
		t.Errorf("selisih emitted = %d, want 80160", got)
	}
}

func TestPipelineFirstPacketNeverPads(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	clk.Advance(2 * time.Second) // pembukaan awal yang lama
	if err := p.HandlePacket(sinePacket(48000, 480, 0, 0), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if p.Emitted() != 160 || p.SilenceGap() != 0 {
		t.Errorf("Emitted=%d SilenceGap=%d, want 160 dan 0", p.Emitted(), p.SilenceGap())
	}
}

func TestPipelineResumeOnlyBeforeFirstPacket(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	clk.Advance(600 * time.Millisecond) // Open memblokir
	p.Resume(clk.Now())
	clk.Advance(100 * time.Millisecond)
	p.Tick(clk.Now())
	if p.Emitted() != 0 {
		t.Errorf("Emitted = %d setelah Resume, want 0 (tanpa sunyi pembuka)", p.Emitted())
	}

	// Setelah ada paket nyata, Resume tidak boleh menghapus celah.
	next := feedSteady(t, p, clk, 48000, 1, 0)
	clk.Advance(time.Second)
	p.Resume(clk.Now())
	if err := p.HandlePacket(sinePacket(48000, 480, next, uint64(next)), clk.Now()); err != nil {
		t.Fatal(err)
	}
	if p.SilenceGap() == 0 {
		t.Error("celah setelah paket nyata harus diisi walau Resume dipanggil")
	}
}

// slowOpener membuat Open kedua memblokir (jam maju) seperti Open sungguhan
// saat format device berubah.
type slowOpener struct {
	*fakeOpener
	clk   *fakeClock
	delay time.Duration
	n     int
}

func (o *slowOpener) Open() (Source, error) {
	o.n++
	if o.n == 2 {
		o.clk.Advance(o.delay)
	}
	return o.fakeOpener.Open()
}

func TestRunFillsReopenGapWithSilence(t *testing.T) {
	p, _, stats, clk := newLoopPipeline(t)
	steps1 := steadySteps(48000, 10, 0, true)
	steps1 = append(steps1, step{
		advance: 10 * time.Millisecond,
		nextErr: &wasapi.HRESULTError{Op: "IAudioCaptureClient::GetBuffer", Code: wasapi.HRDeviceInvalidated},
	})
	s1 := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steps1}
	s2 := &fakeSource{clk: clk, format: f32Format(44100), id: "A", steps: steadySteps(44100, 5, 0, true)}
	inner := &fakeOpener{sources: []*fakeSource{s1, s2}, defaultID: "A"}
	op := &slowOpener{fakeOpener: inner, clk: clk, delay: 1100 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner.exhausted = cancel
	s1.exhausted = cancel
	s2.exhausted = cancel
	ctr := &Counters{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(ctx, op, p, clk, p.cfg, ctr)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("Run tidak berhenti")
	}

	// Celah: 1,1 s (Open) + 10 ms (deteksi galat) + 10 ms (paket pertama stream baru), dikurangi paket itu sendiri.
	if g := p.SilenceGap(); g < 17600 || g > 17800 {
		t.Errorf("SilenceGap = %d, want ~17690", g)
	}
	if p.SilenceIdle() != 0 {
		t.Errorf("SilenceIdle = %d, want 0", p.SilenceIdle())
	}
	if p.Emitted() < 20160 {
		t.Errorf("Emitted = %d, want >= 20160 (linimasa sejajar jam dinding)", p.Emitted())
	}
	gap := time.Duration(ctr.reinitGapNs.Load())
	if gap < 1100*time.Millisecond || gap > 1120*time.Millisecond {
		t.Errorf("celah-reinit = %v, want ~1,1 s", gap)
	}
	if s := stats.Snapshot(); s.Reinits != 1 || s.PositionJumps != 0 {
		t.Errorf("statistik: %+v", s)
	}
}
