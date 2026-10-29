//go:build windows

// Perintah capturetest: uji manual capturer penuh (Milestone 3.1, Tahap 3).
// Menjalankan capture.Capturer, menguras ring 16 kHz mono int16, menulis WAV
// (opsional), dan mencetak ringkasan keadaan tiap detik.
//
// Skenario yang disarankan (putar YouTube di device default):
//
//	go run ./cmd/capturetest -dur 60s -wav ../out-cap.wav
//
//	1. bunyi terus-menerus
//	2. jeda hening 10 detik, lalu bunyi lagi (transisi hening <-> bunyi)
//	3. ganti device default (mis. ke headset Bluetooth) saat berjalan
//	4. ubah format di Sound Settings (mis. 48000 -> 44100 Hz) saat berjalan
//
// Ctrl+C menghentikan lebih awal dengan rapi.
package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"golang.org/x/sys/windows"

	"github.com/master-abror/nizmeet/desktop/internal/audio/capture"
)

func main() { os.Exit(run()) }

func run() int {
	dur := flag.Duration("dur", 30*time.Second, "lama uji")
	wavPath := flag.String("wav", "", "tulis keluaran 16 kHz mono int16 ke berkas WAV")
	idle := flag.Duration("idle", 250*time.Millisecond, "ambang idle (mulai isi sunyi)")
	flag.Parse()

	cfg := capture.DefaultConfig()
	cfg.IdleThreshold = *idle
	c, err := capture.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GAGAL New: %v\n", err)
		return 1
	}
	var w *wavWriter
	if *wavPath != "" {
		if w, err = newWAV(*wavPath, cfg.OutRate); err != nil {
			fmt.Fprintf(os.Stderr, "GAGAL buka WAV: %v\n", err)
			return 1
		}
		defer func() {
			if err := w.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "peringatan: tutup WAV: %v\n", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := c.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "GAGAL Start: %v\n", err)
		return 1
	}
	t0 := time.Now()
	cpu0 := processCPU()

	ring := c.Ring()
	buf := make([]int16, 3200)
	var consumed int64
	maxRing := 0
	drain := func() error {
		for {
			n := ring.Read(buf)
			if n == 0 {
				return nil
			}
			consumed += int64(n)
			if w != nil {
				if err := w.Write(buf[:n]); err != nil {
					return err
				}
			}
		}
	}

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	report := time.NewTicker(time.Second)
	defer report.Stop()
	deadline := time.After(*dur)

	exit := 0
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-deadline:
			break loop
		case <-report.C:
			fmt.Printf("t=%3.0fs  %s\n", time.Since(t0).Seconds(), c.Info())
		case <-tick.C:
			if l := ring.Len(); l > maxRing {
				maxRing = l
			}
			if err := drain(); err != nil {
				fmt.Fprintf(os.Stderr, "GAGAL tulis WAV: %v\n", err)
				exit = 1
				break loop
			}
		}
	}
	c.Stop()
	if err := drain(); err != nil && exit == 0 {
		fmt.Fprintf(os.Stderr, "GAGAL tulis WAV: %v\n", err)
		exit = 1
	}

	wall := time.Since(t0)
	cpu := processCPU() - cpu0
	info := c.Info()
	fmt.Println("---- ringkasan ----")
	fmt.Printf("lama=%v  sampel dikonsumsi=%d (%.2f s @ %d Hz)  sampel keluar pipeline=%d\n",
		wall.Round(time.Millisecond), consumed, float64(consumed)/float64(cfg.OutRate), cfg.OutRate, info.SamplesOut)
	fmt.Printf("rasio keluaran/jam-dinding=%.4f  ring tertinggi=%d/%d sampel\n",
		float64(consumed)/(float64(cfg.OutRate)*wall.Seconds()), maxRing, info.RingCap)
	fmt.Println(info)
	fmt.Printf("CPU proses: %.2f%% dari satu core, %.2f%% ternormalisasi (%d core)\n",
		100*cpu.Seconds()/wall.Seconds(), 100*cpu.Seconds()/wall.Seconds()/float64(runtime.NumCPU()), runtime.NumCPU())
	if info.LastError != "" {
		fmt.Printf("galat terakhir: %s\n", info.LastError)
	}
	return exit
}

func processCPU() time.Duration {
	var c, e, k, u windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &c, &e, &k, &u); err != nil {
		return 0
	}
	return fileTimeDur(k) + fileTimeDur(u)
}

// FILETIME dalam satuan 100 ns.
func fileTimeDur(ft windows.Filetime) time.Duration {
	return time.Duration(int64(ft.HighDateTime)<<32|int64(ft.LowDateTime)) * 100
}

// wavWriter menulis WAV PCM 16-bit mono (header 44 byte, ukuran diisi saat Close).
type wavWriter struct {
	f        *os.File
	dataSize uint32
	scratch  []byte
}

func newWAV(path string, rate int) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	h := make([]byte, 0, 44)
	h = append(h, "RIFF"...)
	h = le.AppendUint32(h, 0) // ukuran RIFF, diisi saat Close
	h = append(h, "WAVEfmt "...)
	h = le.AppendUint32(h, 16)
	h = le.AppendUint16(h, 1) // PCM
	h = le.AppendUint16(h, 1) // mono
	h = le.AppendUint32(h, uint32(rate))
	h = le.AppendUint32(h, uint32(rate*2)) // byte per detik
	h = le.AppendUint16(h, 2)              // block align
	h = le.AppendUint16(h, 16)             // bit per sampel
	h = append(h, "data"...)
	h = le.AppendUint32(h, 0) // ukuran data, diisi saat Close
	if _, err := f.Write(h); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &wavWriter{f: f}, nil
}

func (w *wavWriter) Write(s []int16) error {
	w.scratch = w.scratch[:0]
	for _, v := range s {
		w.scratch = binary.LittleEndian.AppendUint16(w.scratch, uint16(v))
	}
	if _, err := w.f.Write(w.scratch); err != nil {
		return err
	}
	w.dataSize += uint32(len(w.scratch))
	return nil
}

func (w *wavWriter) Close() error {
	le := binary.LittleEndian
	var b [4]byte
	le.PutUint32(b[:], 36+w.dataSize)
	_, err1 := w.f.WriteAt(b[:], 4)
	le.PutUint32(b[:], w.dataSize)
	_, err2 := w.f.WriteAt(b[:], 40)
	return errors.Join(err1, err2, w.f.Close())
}
