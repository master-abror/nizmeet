// Package capture menggabungkan sumber WASAPI (paket wasapi) dengan logika audio
// murni (paket pcm) menjadi capturer loopback yang berjalan sendiri: mode event,
// sunyi sintetis saat hening, ganti device/format, dan keluaran frame 16 kHz
// mono int16 ke ring buffer.
//
// Logika (Pipeline dan Run) tidak bergantung pada perangkat: sumbernya berupa
// antarmuka Source/Opener, sehingga seluruhnya bisa diuji di CI dengan sumber
// palsu. Hanya capture_windows.go yang menyentuh COM.
//
// Paket ini TIDAK boleh mengimpor github.com/wailsapp/wails/v2 (ADR-0001).
package capture

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/pcm"
	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

// Packet adalah satu paket dari perangkat. Data hanya valid sampai Source.Release.
type Packet struct {
	Data           []byte
	Frames         uint32
	DevicePosition uint64
	Silent         bool // isi Data harus diabaikan (anggap nol)
	Discontinuity  bool
	TimestampError bool
}

// Source adalah satu stream loopback yang sedang berjalan pada satu endpoint.
// Semua metode dipanggil dari satu goroutine.
type Source interface {
	Format() wasapi.Format
	DeviceID() string
	// Wait menunggu sinyal data paling lama timeout; signaled=false berarti timeout.
	Wait(timeout time.Duration) (signaled bool, err error)
	// Next mengambil paket berikutnya; ok=false bila tidak ada. Setiap paket
	// yang didapat harus di-Release dengan jumlah frame-nya.
	Next() (pkt Packet, ok bool, err error)
	Release(frames uint32) error
	Close()
}

// Opener membuka stream pada endpoint render default saat ini.
type Opener interface {
	Open() (Source, error)
	// DefaultID mengembalikan ID endpoint default sekarang (deteksi ganti device).
	DefaultID() (string, error)
}

// Clock memungkinkan waktu palsu pada tes.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type systemClock struct{}

func (systemClock) Now() time.Time        { return time.Now() }
func (systemClock) Sleep(d time.Duration) { time.Sleep(d) }

// SystemClock mengembalikan jam sistem.
func SystemClock() Clock { return systemClock{} }

// Config mengatur capturer.
type Config struct {
	OutRate             int           // laju keluaran (Hz)
	FrameSamples        int           // sampel per frame keluaran
	EventTimeout        time.Duration // lama tunggu event; juga granularitas Tick
	IdleThreshold       time.Duration // tanpa paket selama ini = idle (mulai isi sunyi)
	DeviceCheckInterval time.Duration // selang pengecekan endpoint default
	ReopenMin           time.Duration // jeda awal antar percobaan buka ulang
	ReopenMax           time.Duration // batas atas jeda (backoff dua kali lipat)
	MaxCatchUp          time.Duration // batas isi sunyi sekali jalan (setelah macet)
	RingSamples         int           // kapasitas ring keluaran (sampel)
}

// DefaultConfig mengembalikan nilai awal: 16 kHz, frame 20 ms, timeout 20 ms,
// ambang idle 250 ms (di atas celah pengiriman maks ~123 ms yang teramati di
// Tahap 1; akan disetel ulang dari uji 30 menit), ring 5 detik.
func DefaultConfig() Config {
	return Config{
		OutRate:             16000,
		FrameSamples:        pcm.FrameSamples20ms16k,
		EventTimeout:        20 * time.Millisecond,
		IdleThreshold:       250 * time.Millisecond,
		DeviceCheckInterval: time.Second,
		ReopenMin:           100 * time.Millisecond,
		ReopenMax:           2 * time.Second,
		MaxCatchUp:          5 * time.Second,
		RingSamples:         5 * 16000,
	}
}

func (c Config) validate() error {
	switch {
	case c.OutRate <= 0:
		return errors.New("capture: OutRate harus > 0")
	case c.FrameSamples <= 0:
		return errors.New("capture: FrameSamples harus > 0")
	case c.EventTimeout <= 0 || c.DeviceCheckInterval <= 0 || c.ReopenMin <= 0 || c.MaxCatchUp <= 0:
		return errors.New("capture: durasi konfigurasi harus > 0")
	case c.IdleThreshold < c.EventTimeout:
		return errors.New("capture: IdleThreshold tidak boleh lebih kecil dari EventTimeout")
	case c.ReopenMax < c.ReopenMin:
		return errors.New("capture: ReopenMax tidak boleh lebih kecil dari ReopenMin")
	case c.RingSamples < c.FrameSamples:
		return errors.New("capture: RingSamples harus minimal satu frame")
	}
	return nil
}

// Counters adalah penghitung loop (ditulis satu goroutine, dibaca dari mana saja).
type Counters struct {
	reopens          atomic.Uint64
	openFailures     atomic.Uint64
	dataWithoutEvent atomic.Uint64
	reinitGapNs      atomic.Int64
	lastErr          atomic.Pointer[string]
	device           atomic.Pointer[string]
	format           atomic.Pointer[string]
}

func (c *Counters) setErr(err error) {
	if err == nil {
		return
	}
	s := err.Error()
	c.lastErr.Store(&s)
}

func strOf(p *atomic.Pointer[string]) string {
	if v := p.Load(); v != nil {
		return *v
	}
	return ""
}

// Info adalah ringkasan keadaan capturer untuk pemantauan dan alat ukur.
type Info struct {
	Stats pcm.Snapshot

	Device, Format, LastError string
	Idle                      bool

	Reopens          uint64        // stream yang berhenti lalu dibuka ulang
	OpenFailures     uint64        // percobaan buka yang gagal
	ReinitGap        time.Duration // total waktu tanpa stream setelah gangguan
	DataWithoutEvent uint64        // paket yang datang saat event timeout

	SamplesOut      int64  // sampel keluaran total (nyata + sunyi)
	SilenceSamples  int64  // sunyi sintetis
	Clipped         uint64 // sampel yang dijepit saat konversi ke int16
	TimelineResyncs uint64 // isi sunyi dibatasi MaxCatchUp
	DroppedFrames   uint64 // frame utuh dibuang karena ring penuh

	RingLen, RingCap int
}

func (i Info) String() string {
	return fmt.Sprintf("dev=%q fmt=[%s] idle=%v keluar=%d sunyi=%d clip=%d ring=%d/%d buang-frame=%d reopen=%d gagal-buka=%d celah-reinit=%v data-tanpa-event=%d resync=%d | %s",
		i.Device, i.Format, i.Idle, i.SamplesOut, i.SilenceSamples, i.Clipped, i.RingLen, i.RingCap,
		i.DroppedFrames, i.Reopens, i.OpenFailures, i.ReinitGap.Round(time.Millisecond), i.DataWithoutEvent,
		i.TimelineResyncs, i.Stats)
}

func buildInfo(p *Pipeline, st *pcm.Stats, c *Counters, ring *pcm.Ring) Info {
	return Info{
		Stats:            st.Snapshot(),
		Device:           strOf(&c.device),
		Format:           strOf(&c.format),
		LastError:        strOf(&c.lastErr),
		Idle:             p.Idle(),
		Reopens:          c.reopens.Load(),
		OpenFailures:     c.openFailures.Load(),
		ReinitGap:        time.Duration(c.reinitGapNs.Load()),
		DataWithoutEvent: c.dataWithoutEvent.Load(),
		SamplesOut:       p.Emitted(),
		SilenceSamples:   p.SilenceSamples(),
		Clipped:          p.Clipped(),
		TimelineResyncs:  p.Resyncs(),
		DroppedFrames:    p.DroppedFrames(),
		RingLen:          ring.Len(),
		RingCap:          ring.Cap(),
	}
}
