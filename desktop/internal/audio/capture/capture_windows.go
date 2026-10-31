//go:build windows

package capture

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"github.com/master-abror/nizmeet/desktop/internal/audio/pcm"
	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

// wasapiSource adalah stream loopback mode event pada satu endpoint.
type wasapiSource struct {
	dev    *wasapi.Device
	ac     *wasapi.AudioClient
	cc     *wasapi.CaptureClient
	evt    windows.Handle
	format wasapi.Format
	id     string
}

func (s *wasapiSource) Format() wasapi.Format { return s.format }
func (s *wasapiSource) DeviceID() string      { return s.id }

func (s *wasapiSource) init() error {
	var err error
	if s.id, err = s.dev.ID(); err != nil {
		return err
	}
	if s.ac, err = s.dev.ActivateAudioClient(); err != nil {
		return err
	}
	if s.format, _, err = s.ac.MixFormat(); err != nil {
		return err
	}
	if err = s.ac.InitializeLoopback(true); err != nil {
		return err
	}
	if s.evt, err = windows.CreateEvent(nil, 0, 0, nil); err != nil {
		return fmt.Errorf("CreateEvent: %w", err)
	}
	if err = s.ac.SetEventHandle(s.evt); err != nil {
		return err
	}
	if s.cc, err = s.ac.CaptureClient(); err != nil {
		return err
	}
	return s.ac.Start()
}

func (s *wasapiSource) Wait(timeout time.Duration) (bool, error) {
	r, err := windows.WaitForSingleObject(s.evt, uint32(timeout.Milliseconds()))
	if err != nil {
		return false, fmt.Errorf("WaitForSingleObject: %w", err)
	}
	return r != uint32(windows.WAIT_TIMEOUT), nil
}

func (s *wasapiSource) Next() (Packet, bool, error) {
	pk, ok, err := s.cc.GetBuffer()
	if err != nil || !ok {
		return Packet{}, false, err
	}
	return Packet{
		Data:           pk.Data,
		Frames:         pk.Frames,
		DevicePosition: pk.DevicePosition,
		QPCPosition:    pk.QPCPosition,
		Silent:         pk.Flags&wasapi.AudclntBufferflagsSilent != 0,
		Discontinuity:  pk.Flags&wasapi.AudclntBufferflagsDataDiscontinuity != 0,
		TimestampError: pk.Flags&wasapi.AudclntBufferflagsTimestampError != 0,
	}, true, nil
}

func (s *wasapiSource) Release(frames uint32) error { return s.cc.ReleaseBuffer(frames) }

func (s *wasapiSource) Close() {
	if s.ac != nil {
		_ = s.ac.Stop() // galat diabaikan: stream mungkin belum/tidak lagi berjalan
	}
	if s.cc != nil {
		s.cc.Release()
	}
	if s.ac != nil {
		s.ac.Release()
	}
	if s.dev != nil {
		s.dev.Release()
	}
	if s.evt != 0 {
		_ = windows.CloseHandle(s.evt)
	}
}

// opener membuka stream pada endpoint render default.
type opener struct{ enum *wasapi.Enumerator }

func (o *opener) Open() (Source, error) {
	dev, err := o.enum.DefaultRenderDevice()
	if err != nil {
		return nil, err
	}
	s := &wasapiSource{dev: dev}
	if err := s.init(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (o *opener) DefaultID() (string, error) {
	dev, err := o.enum.DefaultRenderDevice()
	if err != nil {
		return "", err
	}
	defer dev.Release()
	return dev.ID()
}

// Capturer menjalankan capture loopback WASAPI di goroutine sendiri (thread OS
// terkunci + COM MTA) dan menyajikan frame 16 kHz mono int16 lewat Ring.
// Start/Stop/Info aman dipanggil dari goroutine mana pun.
type Capturer struct {
	cfg  Config
	ring *pcm.Ring

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	pipe   *Pipeline
	stats  *pcm.Stats
	ctr    *Counters
}

// New membuat Capturer. Belum ada yang berjalan sampai Start dipanggil.
func New(cfg Config) (*Capturer, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	ring, err := pcm.NewRing(cfg.RingSamples)
	if err != nil {
		return nil, err
	}
	return &Capturer{cfg: cfg, ring: ring}, nil
}

// Ring mengembalikan ring keluaran; konsumennya harus satu goroutine.
func (c *Capturer) Ring() *pcm.Ring { return c.ring }

// Start memulai capture. Mengembalikan galat bila COM atau enumerator gagal
// dibuat. Kegagalan membuka device tidak fatal: loop mencoba lagi dengan backoff
// dan melaporkannya lewat Info (OpenFailures, LastError).
func (c *Capturer) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return errors.New("capture: sudah berjalan")
	}
	stats := pcm.NewStats(48000, c.cfg.OutRate) // laju perangkat diganti oleh SetFormat
	pipe, err := NewPipeline(c.cfg, c.ring, stats)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ctr := &Counters{}
	ready := make(chan error, 1)
	done := make(chan struct{})
	go c.run(ctx, pipe, ctr, ready, done)
	if err := <-ready; err != nil {
		cancel()
		<-done
		return err
	}
	c.cancel, c.done = cancel, done
	c.pipe, c.stats, c.ctr = pipe, stats, ctr
	return nil
}

// Stop menghentikan capture dan menunggu goroutine selesai. Aman dipanggil
// berulang. Ekor audio dikuras dan frame terakhir dilengkapi nol.
func (c *Capturer) Stop() {
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.cancel, c.done = nil, nil
	c.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// Info mengembalikan ringkasan keadaan (kosong bila belum pernah Start).
func (c *Capturer) Info() Info {
	c.mu.Lock()
	pipe, stats, ctr := c.pipe, c.stats, c.ctr
	c.mu.Unlock()
	if pipe == nil {
		return Info{}
	}
	return buildInfo(pipe, stats, ctr, c.ring)
}

func (c *Capturer) run(ctx context.Context, pipe *Pipeline, ctr *Counters, ready chan<- error, done chan<- struct{}) {
	defer close(done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	uninit, err := wasapi.InitCOM()
	if err != nil {
		ready <- err
		return
	}
	defer uninit()
	enum, err := wasapi.NewEnumerator()
	if err != nil {
		ready <- err
		return
	}
	defer enum.Release()
	ready <- nil

	Run(ctx, &opener{enum: enum}, pipe, SystemClock(), c.cfg, ctr)
}
