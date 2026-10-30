package capture

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/pcm"
	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

// Pipeline mengubah paket perangkat menjadi frame 16 kHz mono int16 di ring:
// decode -> downmix -> resample -> int16 -> framer -> ring, sambil menjaga
// linimasa (Timeline) dan mengisi sunyi sintetis saat idle.
//
// Aturan linimasa:
//   - Aliran keluaran hanya bertambah di ujung; tidak ada sampel yang ditulis
//     ulang atau dibuang saat transisi hening <-> bunyi (tanpa tumpang-tindih).
//   - Sunyi hanya diisi setelah idle terkonfirmasi (tanpa paket >= IdleThreshold),
//     dan diisi sampai target jam dinding dari paket nyata terakhir (tanpa celah).
//   - Saat masuk idle, resampler dikuras (Flush) agar ekor audio terakhir keluar
//     dan state lama tidak bocor ke audio berikutnya.
//
// Hanya satu goroutine yang boleh memanggil metode mutasi (Begin, SetFormat,
// HandlePacket, Tick, Close). Accessor (Emitted, Idle, ...) aman dari mana saja.
type Pipeline struct {
	cfg    Config
	ring   *pcm.Ring
	stats  *pcm.Stats
	tl     *pcm.Timeline
	framer *pcm.Framer
	emit   func([]int16)

	format  wasapi.Format
	enc     pcm.Encoding
	dm      *pcm.Downmix
	rs      *pcm.Resampler
	rsDirty bool // resampler menerima masukan sejak Reset terakhir

	f32, mono, out []float32
	i16            []int16
	zerosF         []float32
	zerosI         []int16

	lastPacket    time.Time
	idle          bool
	idleAccounted time.Time

	emitted       atomic.Int64
	silence       atomic.Int64 // sunyi pengisi idle (Tick)
	silenceGap    atomic.Int64 // sunyi pengisi celah (buka ulang, macet)
	framePad      atomic.Int64 // nol pelengkap frame saat Close
	everPacket    bool         // sudah ada paket nyata sejak Start
	gaps          GapTracker
	clipped       atomic.Uint64
	resyncs       atomic.Uint64
	droppedFrames atomic.Uint64
	idleFlag      atomic.Bool
}

// NewPipeline membuat pipeline yang menulis frame ke ring dan statistik ke stats.
func NewPipeline(cfg Config, ring *pcm.Ring, stats *pcm.Stats) (*Pipeline, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if ring == nil || stats == nil {
		return nil, errors.New("capture: ring dan stats wajib diisi")
	}
	fr, err := pcm.NewFramer(cfg.FrameSamples)
	if err != nil {
		return nil, err
	}
	tl, err := pcm.NewTimeline(cfg.OutRate)
	if err != nil {
		return nil, err
	}
	p := &Pipeline{cfg: cfg, ring: ring, stats: stats, tl: tl, framer: fr, zerosI: make([]int16, 4096)}
	p.emit = p.writeFrame
	return p, nil
}

// Begin memulai linimasa pada waktu now (sebelum paket pertama).
func (p *Pipeline) Begin(now time.Time) {
	p.lastPacket = now
	p.tl.Anchor(now)
}

func encodingFor(k wasapi.SampleKind) (pcm.Encoding, bool) {
	switch k {
	case wasapi.KindInt16:
		return pcm.EncodingInt16, true
	case wasapi.KindInt24:
		return pcm.EncodingInt24, true
	case wasapi.KindInt32:
		return pcm.EncodingInt32, true
	case wasapi.KindFloat32:
		return pcm.EncodingFloat32, true
	default:
		return pcm.EncodingUnknown, false
	}
}

// SetFormat memasang format stream baru (pertama kali, atau setelah ganti
// device/sample rate). Aliran lama dikuras dulu sehingga ekornya tidak hilang.
// Bila gagal (format tidak didukung), konfigurasi lama tidak berubah.
func (p *Pipeline) SetFormat(f wasapi.Format) error {
	enc, ok := encodingFor(f.Kind)
	if !ok {
		return fmt.Errorf("capture: format tidak didukung: %s", f)
	}
	if f.Channels == 0 || f.SampleRate == 0 || f.BlockAlign == 0 {
		return fmt.Errorf("capture: format tidak valid: %s", f)
	}
	dm, err := pcm.NewDownmix(int(f.Channels), f.ChannelMask)
	if err != nil {
		return err
	}
	rs, err := pcm.NewResampler(int(f.SampleRate), p.cfg.OutRate)
	if err != nil {
		return err
	}
	reconfigure := p.rs != nil
	if reconfigure {
		p.flushResampler()
	}
	p.format, p.enc, p.dm, p.rs = f, enc, dm, rs
	p.gaps.Reset()
	p.stats.SetDeviceRate(int(f.SampleRate))
	if reconfigure {
		p.stats.ObserveReinit()
	}
	return nil
}

// HandlePacket memproses satu paket nyata yang diterima pada waktu now.
// Paket SILENT diperlakukan sebagai nol sebanyak frame-nya (isi buffer diabaikan).
func (p *Pipeline) HandlePacket(pk Packet, now time.Time) error {
	if p.rs == nil {
		return errors.New("capture: format belum diatur")
	}
	if !pk.Silent && len(pk.Data) != int(pk.Frames)*int(p.format.BlockAlign) {
		return fmt.Errorf("capture: panjang paket %d byte tidak sesuai %d frame x %d byte",
			len(pk.Data), pk.Frames, p.format.BlockAlign)
	}
	p.stats.ObservePacket(pcm.PacketInfo{
		Frames:         pk.Frames,
		DevicePosition: pk.DevicePosition,
		Silent:         pk.Silent,
		Discontinuity:  pk.Discontinuity,
		TimestampError: pk.TimestampError,
	})
	p.gaps.Observe(now, pk.Frames, int(p.format.SampleRate), pk.QPCPosition, pk.TimestampError)
	if p.idle {
		p.idle = false
		p.idleFlag.Store(false)
	}
	p.mono = p.mono[:0]
	if pk.Silent {
		p.mono = append(p.mono, p.zeros(int(pk.Frames))...)
	} else {
		var err error
		if p.f32, err = pcm.AppendDecoded(p.f32[:0], pk.Data, p.enc); err != nil {
			return err
		}
		if p.mono, err = p.dm.Apply(p.mono, p.f32); err != nil {
			return err
		}
	}
	p.out = p.rs.Process(p.out[:0], p.mono)
	p.rsDirty = true
	p.catchUp(now, len(p.out))
	p.emitFloat(p.out)
	p.tl.Anchor(now)
	p.lastPacket = now
	p.everPacket = true
	return nil
}

// Tick dipanggil saat satu putaran tunggu selesai tanpa paket (setelah menguras
// sumber). Bila idle sudah terkonfirmasi, mengisi sunyi sampai target jam dinding.
func (p *Pipeline) Tick(now time.Time) {
	if now.Sub(p.lastPacket) < p.cfg.IdleThreshold {
		return
	}
	if !p.idle {
		p.idle = true
		p.idleFlag.Store(true)
		p.idleAccounted = p.lastPacket
		p.flushResampler()
	}
	p.stats.ObserveIdle(now.Sub(p.idleAccounted))
	p.idleAccounted = now

	def := p.tl.Deficit(now)
	if limit := int(int64(p.cfg.MaxCatchUp) * int64(p.cfg.OutRate) / int64(time.Second)); def > limit {
		// Macet panjang (mis. proses ditangguhkan): batasi isi sunyi lalu pasang ulang jangkar.
		p.resyncs.Add(1)
		p.emitSilence(limit, &p.silence)
		p.tl.Anchor(now)
		return
	}
	p.emitSilence(def, &p.silence)
}

// Close menguras ekor resampler dan melengkapi frame yang tersisa dengan nol
// agar aliran berakhir pada batas frame.
func (p *Pipeline) Close() {
	p.flushResampler()
	if pend := p.framer.Pending(); pend > 0 {
		p.emitSilence(p.framer.Size()-pend, &p.framePad)
	}
}

// Emitted mengembalikan total sampel keluaran (nyata + sunyi).
func (p *Pipeline) Emitted() int64 { return p.emitted.Load() }

// SilenceSamples mengembalikan jumlah sunyi sintetis (termasuk pelengkap frame saat Close).
func (p *Pipeline) SilenceSamples() int64 {
	return p.silence.Load() + p.silenceGap.Load() + p.framePad.Load()
}

// Clipped mengembalikan jumlah sampel yang dijepit saat konversi ke int16.
func (p *Pipeline) Clipped() uint64 { return p.clipped.Load() }

// Resyncs mengembalikan berapa kali isi sunyi dibatasi MaxCatchUp.
func (p *Pipeline) Resyncs() uint64 { return p.resyncs.Load() }

// DroppedFrames mengembalikan jumlah frame utuh yang dibuang karena ring penuh.
func (p *Pipeline) DroppedFrames() uint64 { return p.droppedFrames.Load() }

// Idle melaporkan apakah pipeline sedang dalam idle terkonfirmasi.
func (p *Pipeline) Idle() bool { return p.idleFlag.Load() }

func (p *Pipeline) flushResampler() {
	if p.rs == nil || !p.rsDirty {
		return
	}
	p.out = p.rs.Flush(p.out[:0])
	p.rsDirty = false
	p.emitFloat(p.out)
}

func (p *Pipeline) zeros(n int) []float32 {
	if len(p.zerosF) < n {
		p.zerosF = make([]float32, n)
	}
	return p.zerosF[:n]
}

func (p *Pipeline) account(n int) {
	p.tl.AddEmitted(n)
	p.emitted.Add(int64(n))
}

func (p *Pipeline) emitFloat(s []float32) {
	if len(s) == 0 {
		return
	}
	var c int
	p.i16, c = pcm.AppendInt16(p.i16[:0], s)
	p.clipped.Add(uint64(c))
	p.account(len(p.i16))
	p.framer.Push(p.i16, p.emit)
}

func (p *Pipeline) emitSilence(n int, counter *atomic.Int64) {
	for n > 0 {
		k := min(n, len(p.zerosI))
		p.account(k)
		counter.Add(int64(k))
		p.framer.Push(p.zerosI[:k], p.emit)
		n -= k
	}
}

// writeFrame menulis satu frame utuh ke ring. Bila tidak muat, SELURUH frame
// dibuang (bukan sebagian) agar batas frame di ring tetap sejajar.
func (p *Pipeline) writeFrame(frame []int16) {
	if p.ring.Free() < len(frame) {
		p.stats.ObserveRingDrop(len(frame))
		p.droppedFrames.Add(1)
		return
	}
	p.ring.Write(frame)
}
