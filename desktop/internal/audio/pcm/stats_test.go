package pcm

import (
	"testing"
	"time"
)

func feed(st *Stats, n int, frames uint32, startPos uint64) uint64 {
	pos := startPos
	for i := 0; i < n; i++ {
		st.ObservePacket(PacketInfo{Frames: frames, DevicePosition: pos})
		pos += uint64(frames)
	}
	return pos
}

func TestStatsContiguousIsClean(t *testing.T) {
	st := NewStats(48000, 16000)
	feed(st, 100, 480, 0)
	s := st.Snapshot()
	if s.Packets != 100 || s.FramesIn != 48000 || s.PositionJumps != 0 || s.GlitchRatio() != 0 {
		t.Errorf("tidak sesuai: %+v", s)
	}
	if !near(s.ActiveSeconds, 1.0, 1e-9) {
		t.Errorf("ActiveSeconds = %v, want 1", s.ActiveSeconds)
	}
}

func TestStatsPositionJumpCountsAsLoss(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 0})
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 480})
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 1440}) // 480 frame hilang
	s := st.Snapshot()
	if s.PositionJumps != 1 || s.PositionJumpFrames != 480 {
		t.Fatalf("loncatan = %d / %d frame", s.PositionJumps, s.PositionJumpFrames)
	}
	if !near(s.LostSeconds, 0.01, 1e-9) || !near(s.ActiveSeconds, 1920.0/48000.0, 1e-9) {
		t.Errorf("Lost=%v Active=%v", s.LostSeconds, s.ActiveSeconds)
	}
	if !near(s.GlitchRatio(), 0.25, 1e-9) {
		t.Errorf("GlitchRatio = %v, want 0.25", s.GlitchRatio())
	}
}

func TestStatsIdleTransitionIsNotGlitch(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 0})
	st.ObserveIdle(100 * time.Millisecond)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 5280, Discontinuity: true}) // lompat 4800 setelah idle
	s := st.Snapshot()
	if s.PositionJumps != 0 || s.PositionJumpsAfterIdle != 1 || s.PositionJumpFramesAfterIdle != 4800 {
		t.Errorf("loncatan: %+v", s)
	}
	if s.DiscontinuityFlags != 1 || s.DiscontinuityExpected != 1 || s.DiscontinuityUnexplained != 0 {
		t.Errorf("disc: %+v", s)
	}
	if s.LostSeconds != 0 || s.GlitchRatio() != 0 || s.IdleDuration != 100*time.Millisecond {
		t.Errorf("idle dihitung sebagai glitch: %+v", s)
	}
	if !near(s.ActiveSeconds, 960.0/48000.0, 1e-9) {
		t.Errorf("ActiveSeconds = %v", s.ActiveSeconds)
	}
}

func TestStatsDiscontinuityClassification(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 0, Discontinuity: true})                 // tak dijelaskan
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 480, Discontinuity: true, Silent: true}) // paket SILENT: wajar
	s := st.Snapshot()
	if s.DiscontinuityFlags != 2 || s.DiscontinuityUnexplained != 1 || s.DiscontinuityExpected != 1 {
		t.Errorf("klasifikasi: %+v", s)
	}
	if s.SilentPackets != 1 {
		t.Errorf("SilentPackets = %d", s.SilentPackets)
	}
}

func TestStatsRingDropAndRewind(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 0})
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 100}) // mundur
	st.ObserveRingDrop(160)
	st.ObserveRingDrop(0)
	s := st.Snapshot()
	if s.PositionRewinds != 1 || s.RingDropped != 160 || !near(s.LostSeconds, 0.01, 1e-9) {
		t.Errorf("%+v", s)
	}
}

func TestStatsReinitResetsPosition(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 1000})
	st.ObserveReinit()
	st.SetDeviceRate(44100)
	st.ObservePacket(PacketInfo{Frames: 441, DevicePosition: 0, Discontinuity: true})
	s := st.Snapshot()
	if s.Reinits != 1 || s.PositionJumps != 0 || s.PositionRewinds != 0 || s.PositionJumpsAfterIdle != 0 {
		t.Errorf("reinit memicu loncatan palsu: %+v", s)
	}
	if s.DiscontinuityExpected != 1 {
		t.Errorf("disc setelah reinit harus wajar: %+v", s)
	}
	if !near(s.ActiveSeconds, 480.0/48000.0+441.0/44100.0, 1e-9) {
		t.Errorf("ActiveSeconds = %v", s.ActiveSeconds)
	}
}

func TestSnapshotZeroActive(t *testing.T) {
	if r := (Snapshot{}).GlitchRatio(); r != 0 {
		t.Errorf("GlitchRatio kosong = %v", r)
	}
	if s := NewStats(48000, 16000).Snapshot().String(); s == "" {
		t.Error("String kosong")
	}
}
