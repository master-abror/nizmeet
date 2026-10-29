package capture

import (
	"bytes"
	"testing"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

func TestDefaultConfigValid(t *testing.T) {
	if err := DefaultConfig().validate(); err != nil {
		t.Fatal(err)
	}
	bad := DefaultConfig()
	bad.IdleThreshold = time.Millisecond // lebih kecil dari EventTimeout
	if err := bad.validate(); err == nil {
		t.Error("ingin error untuk IdleThreshold < EventTimeout")
	}
}

func TestPipelineSteadyStream(t *testing.T) {
	p, ring, stats, clk := newTestPipeline(t, 48000)
	feedSteady(t, p, clk, 48000, 100, 0) // 1 detik
	if got := p.Emitted(); got != 16000 {
		t.Errorf("Emitted = %d, want 16000", got)
	}
	if ring.Len() != 16000 { // 50 frame utuh
		t.Errorf("ring.Len = %d, want 16000", ring.Len())
	}
	s := stats.Snapshot()
	if s.Packets != 100 || s.PositionJumps != 0 || s.GlitchRatio() != 0 {
		t.Errorf("statistik: %+v", s)
	}
	if p.Idle() || p.SilenceSamples() != 0 || p.Clipped() != 0 {
		t.Errorf("idle=%v silence=%d clipped=%d", p.Idle(), p.SilenceSamples(), p.Clipped())
	}
}

func TestPipelineIdlePaddingTracksWallClock(t *testing.T) {
	p, ring, stats, clk := newTestPipeline(t, 48000)
	feedSteady(t, p, clk, 48000, 50, 0) // 0,5 detik bunyi
	anchor := p.Emitted()
	if anchor != 8000 {
		t.Fatalf("anchor = %d, want 8000", anchor)
	}
	prev := anchor
	for i := 0; i < 50; i++ { // 1 detik tanpa paket, tick tiap 20 ms
		clk.Advance(20 * time.Millisecond)
		p.Tick(clk.Now())
		em := p.Emitted()
		if em < prev {
			t.Fatalf("aliran mundur pada tick %d: %d < %d", i, em, prev)
		}
		if i < 12 && em != anchor { // E = 20..240 ms: belum idle (ambang 250 ms)
			t.Fatalf("tick %d: sunyi disisipkan sebelum idle terkonfirmasi (emitted=%d)", i, em)
		}
		prev = em
	}
	if got := p.Emitted() - anchor; got != 16000 {
		t.Errorf("selisih emitted = %d, want 16000 (1 detik)", got)
	}
	if !p.Idle() {
		t.Error("harus idle")
	}
	if s := p.SilenceSamples(); s < 15800 || s > 16000 { // 16000 dikurangi ekor resampler (~70)
		t.Errorf("SilenceSamples = %d", s)
	}
	if ring.Len() != int(p.Emitted()/320)*320 {
		t.Errorf("ring.Len = %d tidak selaras dengan batas frame", ring.Len())
	}
	idle := stats.Snapshot().IdleDuration
	if idle < 990*time.Millisecond || idle > 1010*time.Millisecond {
		t.Errorf("IdleDuration = %v, want ~1s", idle)
	}
}

func TestPipelineIdleToSoundHasNoExtraPadding(t *testing.T) {
	p, _, stats, clk := newTestPipeline(t, 48000)
	next := feedSteady(t, p, clk, 48000, 50, 0)
	for i := 0; i < 50; i++ {
		clk.Advance(20 * time.Millisecond)
		p.Tick(clk.Now())
	}
	before := p.Emitted()
	clk.Advance(10 * time.Millisecond)
	pk := sinePacket(48000, 480, next, uint64(next))
	pk.Discontinuity = true
	if err := p.HandlePacket(pk, clk.Now()); err != nil {
		t.Fatal(err)
	}
	if got := p.Emitted() - before; got != 160 {
		t.Errorf("paket pertama setelah idle menambah %d sampel, want 160 (tanpa pad tambahan)", got)
	}
	if p.Idle() {
		t.Error("idle harus berakhir saat paket nyata tiba")
	}
	s := stats.Snapshot()
	if s.DiscontinuityExpected != 1 || s.DiscontinuityUnexplained != 0 || s.PositionJumps != 0 || s.GlitchRatio() != 0 {
		t.Errorf("transisi idle dihitung glitch: %+v", s)
	}
}

func TestPipelineSilentPacketsBecomeZeros(t *testing.T) {
	p, ring, stats, clk := newTestPipeline(t, 48000)
	garbage := bytes.Repeat([]byte{0xFF}, 480*8) // NaN bila dibaca sebagai float32
	for i := 0; i < 100; i++ {
		clk.Advance(10 * time.Millisecond)
		pk := Packet{Data: garbage, Frames: 480, DevicePosition: uint64(i * 480), Silent: true}
		if err := p.HandlePacket(pk, clk.Now()); err != nil {
			t.Fatal(err)
		}
	}
	out := make([]int16, ring.Len())
	if n := ring.Read(out); n != 16000 {
		t.Fatalf("terbaca %d sampel, want 16000", n)
	}
	for i, v := range out {
		if v != 0 {
			t.Fatalf("sampel %d = %d, want 0", i, v)
		}
	}
	if s := stats.Snapshot(); s.SilentPackets != 100 {
		t.Errorf("SilentPackets = %d", s.SilentPackets)
	}
}

func TestPipelineFormatChange(t *testing.T) {
	p, _, stats, clk := newTestPipeline(t, 48000)
	feedSteady(t, p, clk, 48000, 10, 0) // 1600 sampel
	if err := p.SetFormat(f32Format(44100)); err != nil {
		t.Fatal(err)
	}
	feedSteady(t, p, clk, 44100, 10, 0) // posisi perangkat mulai dari awal
	em := p.Emitted()
	// 1600 + ekor resampler lama (~70) + 10 x 160
	if em < 3200 || em > 3300 {
		t.Errorf("Emitted = %d, want ~3270", em)
	}
	s := stats.Snapshot()
	if s.Reinits != 1 || s.PositionJumps != 0 || s.PositionRewinds != 0 {
		t.Errorf("statistik setelah ganti format: %+v", s)
	}
}

func TestPipelineRejectsBadFormat(t *testing.T) {
	p, _, _, _ := newTestPipeline(t, 48000)
	bad := f32Format(48000)
	bad.Kind = wasapi.KindUnknown
	if err := p.SetFormat(bad); err == nil {
		t.Error("ingin error untuk format tak dikenal")
	}
	if err := p.SetFormat(f32Format(44101)); err == nil {
		t.Error("ingin error untuk rasio yang tidak didukung resampler")
	}
	// konfigurasi lama tetap berfungsi
	clk := newFakeClock()
	if err := p.HandlePacket(sinePacket(48000, 480, 0, 0), clk.Now()); err != nil {
		t.Errorf("konfigurasi lama rusak setelah SetFormat gagal: %v", err)
	}
	if err := p.HandlePacket(Packet{Data: make([]byte, 10), Frames: 480}, clk.Now()); err == nil {
		t.Error("ingin error untuk panjang paket yang tidak sesuai")
	}
}

func TestPipelineRingOverrunDropsWholeFrames(t *testing.T) {
	p, ring, stats, clk := newTestPipelineRing(t, 48000, 960) // 3 frame
	feedSteady(t, p, clk, 48000, 100, 0)                      // 50 frame diproduksi
	if ring.Len() != 960 {
		t.Errorf("ring.Len = %d, want 960", ring.Len())
	}
	if got := p.DroppedFrames(); got != 47 {
		t.Errorf("DroppedFrames = %d, want 47", got)
	}
	if got := stats.Snapshot().RingDropped; got != 47*320 {
		t.Errorf("RingDropped = %d, want %d", got, 47*320)
	}
}

func TestPipelineMaxCatchUpResyncs(t *testing.T) {
	p, _, _, clk := newTestPipeline(t, 48000)
	feedSteady(t, p, clk, 48000, 10, 0)
	anchor := p.Emitted()
	clk.Advance(60 * time.Second) // proses macet 1 menit
	p.Tick(clk.Now())
	if p.Resyncs() != 1 {
		t.Errorf("Resyncs = %d, want 1", p.Resyncs())
	}
	if got := p.Emitted() - anchor; got < 80000 || got > 80100 { // 5 detik + ekor resampler
		t.Errorf("sunyi yang dikeluarkan = %d, want ~80000", got)
	}
}

func TestPipelineCloseAlignsToFrameBoundary(t *testing.T) {
	p, ring, _, clk := newTestPipeline(t, 48000)
	feedSteady(t, p, clk, 48000, 7, 0) // 1120 sampel = 3,5 frame
	p.Close()
	if ring.Len()%320 != 0 || ring.Len() < 1120 {
		t.Errorf("ring.Len = %d setelah Close, want kelipatan 320 dan >= 1120", ring.Len())
	}
}
