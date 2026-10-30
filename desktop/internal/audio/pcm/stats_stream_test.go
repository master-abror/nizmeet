package pcm

import "testing"

func TestStatsFirstPacketOfStreamDiscontinuityIsExpected(t *testing.T) {
	st := NewStats(48000, 16000)
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 0, Discontinuity: true}) // awal stream
	s := st.Snapshot()
	if s.DiscontinuityExpected != 1 || s.DiscontinuityUnexplained != 0 {
		t.Fatalf("flag di paket pertama stream harus wajar: %+v", s)
	}
	st.ObservePacket(PacketInfo{Frames: 480, DevicePosition: 480, Discontinuity: true}) // tengah aliran
	s = st.Snapshot()
	if s.DiscontinuityExpected != 1 || s.DiscontinuityUnexplained != 1 {
		t.Errorf("flag di tengah aliran harus tak dijelaskan: %+v", s)
	}
}
