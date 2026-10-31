package capture

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/master-abror/nizmeet/desktop/internal/audio/pcm"
	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

// fakeClock adalah jam palsu: Sleep memajukan waktu, tanpa menunggu sungguhan.
type fakeClock struct{ now time.Time }

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(1_000_000, 0)} }

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }
func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func f32Format(rate int) wasapi.Format {
	return wasapi.Format{
		Tag:            wasapi.WaveFormatExtensible,
		EffectiveTag:   wasapi.WaveFormatIEEEFloat,
		Channels:       2,
		SampleRate:     uint32(rate),
		AvgBytesPerSec: uint32(rate * 8),
		BlockAlign:     8,
		BitsPerSample:  32,
		ValidBits:      32,
		ChannelMask:    0x3,
		Extensible:     true,
		Kind:           wasapi.KindFloat32,
	}
}

// sinePacket membuat paket stereo float32 berisi sinus 440 Hz yang kontinu antarpaket.
func sinePacket(rate, frames, startFrame int, pos uint64) Packet {
	data := make([]byte, 0, frames*8)
	for i := 0; i < frames; i++ {
		v := float32(0.3 * math.Sin(2*math.Pi*440*float64(startFrame+i)/float64(rate)))
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(v))
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(v))
	}
	return Packet{Data: data, Frames: uint32(frames), DevicePosition: pos}
}

func testConfig() Config {
	c := DefaultConfig()
	c.RingSamples = 200000
	return c
}

func newTestPipelineRing(t *testing.T, rate, ringSize int) (*Pipeline, *pcm.Ring, *pcm.Stats, *fakeClock) {
	t.Helper()
	cfg := testConfig()
	cfg.RingSamples = ringSize
	ring, err := pcm.NewRing(ringSize)
	if err != nil {
		t.Fatal(err)
	}
	stats := pcm.NewStats(48000, cfg.OutRate)
	p, err := NewPipeline(cfg, ring, stats)
	if err != nil {
		t.Fatal(err)
	}
	clk := newFakeClock()
	p.Begin(clk.Now())
	if err := p.SetFormat(f32Format(rate)); err != nil {
		t.Fatal(err)
	}
	return p, ring, stats, clk
}

// newLoopPipeline membuat pipeline tanpa Begin/SetFormat: keduanya dilakukan Run.
func newLoopPipeline(t *testing.T) (*Pipeline, *pcm.Ring, *pcm.Stats, *fakeClock) {
	t.Helper()
	cfg := testConfig()
	ring, err := pcm.NewRing(cfg.RingSamples)
	if err != nil {
		t.Fatal(err)
	}
	stats := pcm.NewStats(48000, cfg.OutRate)
	p, err := NewPipeline(cfg, ring, stats)
	if err != nil {
		t.Fatal(err)
	}
	return p, ring, stats, newFakeClock()
}

func newTestPipeline(t *testing.T, rate int) (*Pipeline, *pcm.Ring, *pcm.Stats, *fakeClock) {
	t.Helper()
	return newTestPipelineRing(t, rate, 200000)
}

// feedSteady memasukkan n paket 10 ms yang kontinu, memajukan jam 10 ms per paket.
// Mengembalikan indeks frame berikutnya.
func feedSteady(t *testing.T, p *Pipeline, clk *fakeClock, rate, n, startFrame int) int {
	t.Helper()
	frames := rate / 100
	for i := 0; i < n; i++ {
		clk.Advance(10 * time.Millisecond)
		pk := sinePacket(rate, frames, startFrame, uint64(startFrame))
		if err := p.HandlePacket(pk, clk.Now()); err != nil {
			t.Fatal(err)
		}
		startFrame += frames
	}
	return startFrame
}

// ---- sumber dan opener palsu untuk tes Run ----

type step struct {
	advance  time.Duration
	signaled bool
	packets  []Packet
	nextErr  error
	hook     func()
}

type fakeSource struct {
	clk       *fakeClock
	format    wasapi.Format
	id        string
	steps     []step
	exhausted func()

	pend    []Packet
	nextErr error
	closed  bool
}

func (s *fakeSource) Format() wasapi.Format { return s.format }
func (s *fakeSource) DeviceID() string      { return s.id }

func (s *fakeSource) Wait(timeout time.Duration) (bool, error) {
	if len(s.steps) == 0 {
		s.clk.Advance(timeout)
		s.exhausted()
		return false, nil
	}
	st := s.steps[0]
	s.steps = s.steps[1:]
	s.clk.Advance(st.advance)
	if st.hook != nil {
		st.hook()
	}
	s.pend, s.nextErr = st.packets, st.nextErr
	return st.signaled, nil
}

func (s *fakeSource) Next() (Packet, bool, error) {
	if s.nextErr != nil {
		err := s.nextErr
		s.nextErr = nil
		return Packet{}, false, err
	}
	if len(s.pend) == 0 {
		return Packet{}, false, nil
	}
	pk := s.pend[0]
	s.pend = s.pend[1:]
	return pk, true, nil
}

func (s *fakeSource) Release(uint32) error { return nil }
func (s *fakeSource) Close()               { s.closed = true }

type fakeOpener struct {
	sources   []*fakeSource
	openErrs  []error
	defaultID string
	opens     int
	exhausted func()
}

func (o *fakeOpener) Open() (Source, error) {
	o.opens++
	if len(o.openErrs) > 0 {
		e := o.openErrs[0]
		o.openErrs = o.openErrs[1:]
		return nil, e
	}
	if len(o.sources) == 0 {
		o.exhausted()
		return nil, errors.New("tidak ada sumber lagi")
	}
	s := o.sources[0]
	o.sources = o.sources[1:]
	return s, nil
}

func (o *fakeOpener) DefaultID() (string, error) { return o.defaultID, nil }

// steadySteps membuat n langkah: tiap langkah satu paket 10 ms yang kontinu.
func steadySteps(rate, n, startFrame int, signaled bool) []step {
	frames := rate / 100
	steps := make([]step, 0, n)
	for i := 0; i < n; i++ {
		pk := sinePacket(rate, frames, startFrame, uint64(startFrame))
		steps = append(steps, step{advance: 10 * time.Millisecond, signaled: signaled, packets: []Packet{pk}})
		startFrame += frames
	}
	return steps
}
