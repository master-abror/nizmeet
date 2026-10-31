package capture

import (
	"sync"
	"time"
)

// Parameter analisis celah antarpaket.
const (
	bigGap        = 50 * time.Millisecond // celah di atas ini diklasifikasi (jitter/hening/lain)
	contiguousTol = 5 * time.Millisecond  // selisih jam capture di bawah ini = audio kontinu
	qpcUnit       = 100 * time.Nanosecond // satuan QPCPosition; TIDAK TERVERIFIKASI (dokumentasi, bukan header lampiran)
)

// GapBucketLabels memberi label tiap bucket histogram celah (batas atas inklusif).
var GapBucketLabels = [7]string{"<=15ms", "<=30ms", "<=50ms", "<=100ms", "<=250ms", "<=1s", ">1s"}

var gapBucketLimits = [6]time.Duration{
	15 * time.Millisecond, 30 * time.Millisecond, 50 * time.Millisecond,
	100 * time.Millisecond, 250 * time.Millisecond, time.Second,
}

// GapSnapshot adalah ringkasan celah antarpaket (jam dinding, saat paket tiba).
//
// Klasifikasi celah > 50 ms memakai QPCPosition (jam capture perangkat):
//   - Jitter: jam capture kontinu (audio tidak hilang), hanya pengirimannya terlambat.
//   - Silence: jam capture ikut berlubang kira-kira selama celah (perangkat memang
//     tidak menghasilkan audio selama itu).
//   - Other: di antara keduanya.
//   - Unknown: QPCPosition tidak tersedia (0) atau berflag TIMESTAMP_ERROR.
type GapSnapshot struct {
	Count          uint64 // celah > 50 ms yang diklasifikasi
	Max            time.Duration
	Hist           [7]uint64
	Jitter         uint64
	Silence        uint64
	Other          uint64
	Unknown        uint64
	MaxJitter      time.Duration // celah jam dinding terbesar yang tergolong jitter
	MaxSilenceHole time.Duration // lubang jam capture terbesar yang tergolong hening
	QPCSpan        time.Duration // jumlah selisih QPC antarpaket (anggap 100 ns/satuan)
	ArrivalSpan    time.Duration // jumlah selisih jam dinding untuk pasangan paket yang sama
}

// QPCRatio membandingkan rentang QPC dengan rentang jam dinding pada pasangan
// paket yang sama. Harus mendekati 1,000; bila jauh, asumsi satuan QPC salah.
// 0 bila belum ada data.
func (s GapSnapshot) QPCRatio() float64 {
	if s.ArrivalSpan <= 0 {
		return 0
	}
	return float64(s.QPCSpan) / float64(s.ArrivalSpan)
}

// GapTracker mengukur celah antarpaket. Aman dipakai dari beberapa goroutine.
type GapTracker struct {
	mu          sync.Mutex
	have        bool
	prevArrival time.Time
	prevQPC     uint64
	prevDur     time.Duration
	s           GapSnapshot
	intervalMax time.Duration
}

// Reset melupakan paket sebelumnya (stream baru) tanpa menghapus statistik kumulatif.
func (g *GapTracker) Reset() {
	g.mu.Lock()
	g.have = false
	g.mu.Unlock()
}

// Snapshot mengembalikan statistik kumulatif.
func (g *GapTracker) Snapshot() GapSnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.s
}

// TakeIntervalMax mengembalikan celah terbesar sejak pemanggilan sebelumnya lalu meresetnya.
func (g *GapTracker) TakeIntervalMax() time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	m := g.intervalMax
	g.intervalMax = 0
	return m
}

func bucketIndex(gap time.Duration) int {
	for i, lim := range gapBucketLimits {
		if gap <= lim {
			return i
		}
	}
	return len(gapBucketLimits)
}

// Observe mencatat satu paket: waktu tiba, jumlah frame, laju perangkat, dan QPCPosition.
func (g *GapTracker) Observe(arrival time.Time, frames uint32, rate int, qpc uint64, tsErr bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var dur time.Duration
	if rate > 0 {
		dur = time.Duration(int64(frames) * int64(time.Second) / int64(rate))
	}
	if g.have {
		gap := arrival.Sub(g.prevArrival)
		g.s.Hist[bucketIndex(gap)]++
		if gap > g.s.Max {
			g.s.Max = gap
		}
		if gap > g.intervalMax {
			g.intervalMax = gap
		}
		qpcOK := qpc != 0 && g.prevQPC != 0 && !tsErr && qpc >= g.prevQPC
		var qpcDelta time.Duration
		if qpcOK {
			qpcDelta = time.Duration(qpc-g.prevQPC) * qpcUnit
			g.s.QPCSpan += qpcDelta
			g.s.ArrivalSpan += gap
		}
		if gap > bigGap {
			g.s.Count++
			if !qpcOK {
				g.s.Unknown++
			} else {
				hole := qpcDelta - g.prevDur // waktu tanpa audio terekam di antara dua paket
				switch {
				case hole < contiguousTol:
					g.s.Jitter++
					if gap > g.s.MaxJitter {
						g.s.MaxJitter = gap
					}
				case hole*2 > gap:
					g.s.Silence++
					if hole > g.s.MaxSilenceHole {
						g.s.MaxSilenceHole = hole
					}
				default:
					g.s.Other++
				}
			}
		}
	}
	g.have, g.prevArrival, g.prevQPC, g.prevDur = true, arrival, qpc, dur
}
