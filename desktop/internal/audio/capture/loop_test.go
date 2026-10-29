package capture

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

// runLoop menjalankan Run di goroutine terpisah dan menunggu sampai semua sumber
// palsu habis (yang membatalkan konteks). Jam palsu hanya disentuh goroutine itu.
func runLoop(t *testing.T, op *fakeOpener, p *Pipeline, clk *fakeClock) *Counters {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op.exhausted = cancel
	for _, s := range op.sources {
		s.exhausted = cancel
	}
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
	return ctr
}

func TestRunSteadyStreamThenStop(t *testing.T) {
	p, ring, stats, clk := newLoopPipeline(t)
	src := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steadySteps(48000, 20, 0, true)}
	op := &fakeOpener{sources: []*fakeSource{src}, defaultID: "A"}
	ctr := runLoop(t, op, p, clk)

	if ctr.reopens.Load() != 0 || ctr.openFailures.Load() != 0 {
		t.Errorf("reopens=%d openFailures=%d", ctr.reopens.Load(), ctr.openFailures.Load())
	}
	if s := stats.Snapshot(); s.Packets != 20 || s.PositionJumps != 0 {
		t.Errorf("statistik: %+v", s)
	}
	if p.Emitted() < 3200 {
		t.Errorf("Emitted = %d, want >= 3200", p.Emitted())
	}
	if ring.Len()%320 != 0 { // Close melengkapi frame terakhir
		t.Errorf("ring.Len = %d tidak selaras batas frame setelah berhenti", ring.Len())
	}
	if !src.closed {
		t.Error("sumber harus ditutup")
	}
	if got := strOf(&ctr.device); got != "A" {
		t.Errorf("device = %q", got)
	}
}

func TestRunReopensAfterDeviceInvalidated(t *testing.T) {
	p, _, stats, clk := newLoopPipeline(t)
	steps1 := steadySteps(48000, 5, 0, true)
	steps1 = append(steps1, step{
		advance: 10 * time.Millisecond,
		nextErr: &wasapi.HRESULTError{Op: "IAudioCaptureClient::GetBuffer", Code: wasapi.HRDeviceInvalidated},
	})
	s1 := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steps1}
	s2 := &fakeSource{clk: clk, format: f32Format(44100), id: "A", steps: steadySteps(44100, 5, 0, true)}
	op := &fakeOpener{sources: []*fakeSource{s1, s2}, defaultID: "A"}
	ctr := runLoop(t, op, p, clk)

	if ctr.reopens.Load() != 1 {
		t.Errorf("reopens = %d, want 1", ctr.reopens.Load())
	}
	if !s1.closed || !s2.closed {
		t.Errorf("closed: s1=%v s2=%v", s1.closed, s2.closed)
	}
	s := stats.Snapshot()
	if s.Packets != 10 || s.Reinits != 1 || s.PositionJumps != 0 || s.PositionRewinds != 0 {
		t.Errorf("statistik: %+v", s)
	}
	if got := strOf(&ctr.lastErr); !strings.Contains(got, "AUDCLNT_E_DEVICE_INVALIDATED") {
		t.Errorf("lastErr = %q", got)
	}
}

func TestRunRetriesOpenWithBackoff(t *testing.T) {
	p, _, _, clk := newLoopPipeline(t)
	src := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steadySteps(48000, 5, 0, true)}
	op := &fakeOpener{
		sources:   []*fakeSource{src},
		openErrs:  []error{&wasapi.HRESULTError{Op: "x", Code: 0x80070005}, &wasapi.HRESULTError{Op: "y", Code: 0x80070005}},
		defaultID: "A",
	}
	ctr := runLoop(t, op, p, clk)

	if ctr.openFailures.Load() != 2 {
		t.Errorf("openFailures = %d, want 2", ctr.openFailures.Load())
	}
	if ctr.reopens.Load() != 0 {
		t.Errorf("reopens = %d, want 0 (belum pernah streaming sebelum gagal)", ctr.reopens.Load())
	}
	// backoff 100 ms lalu 200 ms, dalam potongan 20 ms
	gap := time.Duration(ctr.reinitGapNs.Load())
	if gap < 300*time.Millisecond || gap > 320*time.Millisecond {
		t.Errorf("celah = %v, want ~300ms", gap)
	}
	if p.Emitted() == 0 {
		t.Error("linimasa harus tetap berjalan (sunyi) selama menunggu device")
	}
}

func TestRunDetectsDefaultDeviceChange(t *testing.T) {
	p, _, _, clk := newLoopPipeline(t)
	op := &fakeOpener{defaultID: "A"}
	steps1 := make([]step, 20)
	for i := range steps1 {
		steps1[i] = step{advance: 100 * time.Millisecond}
	}
	steps1[3].hook = func() { op.defaultID = "B" }
	s1 := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steps1}
	s2 := &fakeSource{clk: clk, format: f32Format(48000), id: "B"}
	op.sources = []*fakeSource{s1, s2}
	ctr := runLoop(t, op, p, clk)

	if ctr.reopens.Load() != 1 || op.opens != 2 {
		t.Errorf("reopens=%d opens=%d, want 1 dan 2", ctr.reopens.Load(), op.opens)
	}
	if !s1.closed {
		t.Error("sumber lama harus ditutup")
	}
	if got := strOf(&ctr.device); got != "B" {
		t.Errorf("device = %q, want B", got)
	}
	if got := strOf(&ctr.lastErr); !strings.Contains(got, "berganti") {
		t.Errorf("lastErr = %q", got)
	}
}

func TestRunCountsDataWithoutEvent(t *testing.T) {
	p, _, _, clk := newLoopPipeline(t)
	src := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steadySteps(48000, 3, 0, false)}
	op := &fakeOpener{sources: []*fakeSource{src}, defaultID: "A"}
	ctr := runLoop(t, op, p, clk)
	if got := ctr.dataWithoutEvent.Load(); got != 3 {
		t.Errorf("dataWithoutEvent = %d, want 3", got)
	}
}

func TestRunKeepsTimelineWhileSilent(t *testing.T) {
	p, _, _, clk := newLoopPipeline(t)
	steps := make([]step, 50) // 1 detik tanpa paket, 20 ms per langkah
	for i := range steps {
		steps[i] = step{advance: 20 * time.Millisecond}
	}
	src := &fakeSource{clk: clk, format: f32Format(48000), id: "A", steps: steps}
	op := &fakeOpener{sources: []*fakeSource{src}, defaultID: "A"}
	runLoop(t, op, p, clk)
	// 1 detik penuh + satu putaran tunggu terakhir (20 ms) sebelum berhenti.
	if got := p.Emitted(); got < 16000 || got > 16320 {
		t.Errorf("Emitted = %d, want 16000..16320", got)
	}
	if !p.Idle() {
		t.Error("harus idle")
	}
}
