package pcm

import (
	"fmt"
	"sync"
	"time"
)

// PacketInfo adalah ringkasan satu paket dari perangkat (tanpa data audio).
type PacketInfo struct {
	Frames         uint32 // frame perangkat dalam paket
	DevicePosition uint64 // posisi frame pertama menurut perangkat
	Silent         bool   // flag AUDCLNT_BUFFERFLAGS_SILENT
	Discontinuity  bool   // flag AUDCLNT_BUFFERFLAGS_DATA_DISCONTINUITY
	TimestampError bool   // flag AUDCLNT_BUFFERFLAGS_TIMESTAMP_ERROR
}

// Snapshot adalah salinan statistik pada satu waktu.
type Snapshot struct {
	Packets       uint64
	FramesIn      uint64 // frame perangkat yang diterima
	SilentPackets uint64
	IdleDuration  time.Duration // total waktu tanpa paket (loopback diam)
	Reinits       uint64        // inisialisasi ulang (ganti device/format)

	PositionJumps               uint64 // loncatan maju posisi di tengah aliran aktif
	PositionJumpFrames          uint64 // frame perangkat yang hilang pada loncatan tsb
	PositionJumpsAfterIdle      uint64 // loncatan maju tepat setelah idle (tidak dianggap glitch)
	PositionJumpFramesAfterIdle uint64
	PositionRewinds             uint64 // posisi mundur (anomali; tidak masuk rasio)

	DiscontinuityFlags       uint64 // semua flag DATA_DISCONTINUITY
	DiscontinuityExpected    uint64 // pada paket pertama setelah idle atau paket SILENT
	DiscontinuityUnexplained uint64 // di tengah aliran aktif tanpa penjelasan
	TimestampErrors          uint64

	RingDropped uint64 // sampel keluaran (laju keluaran) dibuang karena ring penuh

	ActiveSeconds float64 // frame diterima + frame hilang (tanpa idle), dalam detik
	LostSeconds   float64 // frame hilang (loncatan posisi) + sampel ring terbuang, dalam detik
}

// GlitchRatio adalah LostSeconds/ActiveSeconds (0 bila belum ada audio aktif).
//
// Definisi glitch (direvisi dari hasil uji perangkat Tahap 1): DATA_DISCONTINUITY
// sendiri BUKAN glitch karena muncul normal setelah hening. Yang dihitung hanya
// frame hilang menurut loncatan posisi perangkat di tengah aliran aktif dan
// sampel yang terbuang karena overrun ring buffer. Celah akibat inisialisasi
// ulang (ganti device) dan periode idle dicatat terpisah.
func (s Snapshot) GlitchRatio() float64 {
	if s.ActiveSeconds <= 0 {
		return 0
	}
	return s.LostSeconds / s.ActiveSeconds
}

func (s Snapshot) String() string {
	return fmt.Sprintf(
		"paket=%d frame=%d silent=%d idle=%v reinit=%d | loncatan=%d (%d frame) setelah-idle=%d mundur=%d | disc=%d (wajar=%d, tak-jelas=%d) tserr=%d | ring-buang=%d | glitch=%.4f%% (hilang %.3fs dari %.3fs)",
		s.Packets, s.FramesIn, s.SilentPackets, s.IdleDuration.Round(time.Millisecond), s.Reinits,
		s.PositionJumps, s.PositionJumpFrames, s.PositionJumpsAfterIdle, s.PositionRewinds,
		s.DiscontinuityFlags, s.DiscontinuityExpected, s.DiscontinuityUnexplained, s.TimestampErrors,
		s.RingDropped, s.GlitchRatio()*100, s.LostSeconds, s.ActiveSeconds)
}

// Stats mengumpulkan statistik capture. Aman dipakai dari beberapa goroutine.
type Stats struct {
	mu            sync.Mutex
	s             Snapshot
	deviceRate    int
	outRate       int
	expectedPos   uint64
	havePos       bool
	idleSinceLast bool
}

// NewStats membuat penghitung untuk laju perangkat dan laju keluaran (Hz).
func NewStats(deviceRate, outRate int) *Stats {
	return &Stats{deviceRate: deviceRate, outRate: outRate}
}

// SetDeviceRate mengganti laju perangkat (mis. setelah ganti format); panggil bersama ObserveReinit.
func (st *Stats) SetDeviceRate(rate int) {
	st.mu.Lock()
	st.deviceRate = rate
	st.mu.Unlock()
}

// ObserveIdle mencatat periode tanpa paket sepanjang d. Paket berikutnya
// diperlakukan sebagai "tepat setelah idle".
func (st *Stats) ObserveIdle(d time.Duration) {
	st.mu.Lock()
	st.s.IdleDuration += d
	st.idleSinceLast = true
	st.mu.Unlock()
}

// ObserveReinit mencatat inisialisasi ulang stream. Posisi perangkat dianggap
// mulai dari awal, jadi tidak ada loncatan palsu pada paket berikutnya.
func (st *Stats) ObserveReinit() {
	st.mu.Lock()
	st.s.Reinits++
	st.havePos = false
	st.idleSinceLast = true
	st.mu.Unlock()
}

// ObserveRingDrop mencatat n sampel keluaran yang terbuang karena ring penuh.
func (st *Stats) ObserveRingDrop(n int) {
	if n <= 0 {
		return
	}
	st.mu.Lock()
	st.s.RingDropped += uint64(n)
	st.s.LostSeconds += float64(n) / float64(st.outRate)
	st.mu.Unlock()
}

// ObservePacket mencatat satu paket dari perangkat.
func (st *Stats) ObservePacket(p PacketInfo) {
	st.mu.Lock()
	defer st.mu.Unlock()
	s := &st.s
	rate := float64(st.deviceRate)

	s.Packets++
	s.FramesIn += uint64(p.Frames)
	s.ActiveSeconds += float64(p.Frames) / rate
	if p.Silent {
		s.SilentPackets++
	}
	afterIdle := st.idleSinceLast
	firstOfStream := !st.havePos // paket pertama stream (baru atau setelah reinit)
	st.idleSinceLast = false

	if st.havePos {
		switch {
		case p.DevicePosition > st.expectedPos:
			missing := p.DevicePosition - st.expectedPos
			if afterIdle {
				s.PositionJumpsAfterIdle++
				s.PositionJumpFramesAfterIdle += missing
			} else {
				s.PositionJumps++
				s.PositionJumpFrames += missing
				s.LostSeconds += float64(missing) / rate
				s.ActiveSeconds += float64(missing) / rate
			}
		case p.DevicePosition < st.expectedPos:
			s.PositionRewinds++
		}
	}
	st.expectedPos = p.DevicePosition + uint64(p.Frames)
	st.havePos = true

	if p.Discontinuity {
		s.DiscontinuityFlags++
		if afterIdle || firstOfStream || p.Silent {
			s.DiscontinuityExpected++
		} else {
			s.DiscontinuityUnexplained++
		}
	}
	if p.TimestampError {
		s.TimestampErrors++
	}
}

// Snapshot mengembalikan salinan statistik saat ini.
func (st *Stats) Snapshot() Snapshot {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}
