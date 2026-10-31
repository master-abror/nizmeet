//go:build windows

// Perintah audiocap: prototipe dan alat ukur manual WASAPI loopback (Milestone 3.1).
// Dijalankan manual oleh Kontributor; tidak dipakai di CI.
//
// Tahap 1: langkah bertingkat untuk menguji satu antarmuka COM per langkah.
//
//	-step 1  COM init, enumerator, endpoint render default (ID, state)
//	-step 2  + IAudioClient, mix format, periode device
//	-step 3  + Initialize(loopback), buffer size, IAudioCaptureClient
//	-step 4  + Start, capture -dur, statistik per detik, ringkasan (default)
//
// Contoh:
//
//	go run ./cmd/audiocap -step 2
//	go run ./cmd/audiocap -mode poll  -dur 10s -wav out-poll.wav
//	go run ./cmd/audiocap -mode event -dur 10s -wav out-event.wav
package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"golang.org/x/sys/windows"

	"github.com/master-abror/nizmeet/desktop/internal/audio/wasapi"
)

func init() {
	// Semua panggilan COM harus terjadi di satu thread OS.
	runtime.LockOSThread()
}

func main() { os.Exit(run()) }

func fail(what string, err error) int {
	fmt.Fprintf(os.Stderr, "GAGAL %s: %v\n", what, err)
	return 1
}

func run() int {
	step := flag.Int("step", 4, "berhenti setelah langkah ini (1-4)")
	mode := flag.String("mode", "poll", "poll | event")
	dur := flag.Duration("dur", 10*time.Second, "lama capture pada langkah 4")
	wavPath := flag.String("wav", "", "tulis audio format asli (mix format) ke berkas WAV")
	flag.Parse()
	if *mode != "poll" && *mode != "event" {
		fmt.Fprintln(os.Stderr, "-mode harus poll atau event")
		return 2
	}
	eventMode := *mode == "event"

	// --- Langkah 1 ---
	uninit, err := wasapi.InitCOM()
	if err != nil {
		return fail("langkah 1 InitCOM", err)
	}
	defer uninit()
	enum, err := wasapi.NewEnumerator()
	if err != nil {
		return fail("langkah 1 NewEnumerator", err)
	}
	defer enum.Release()
	dev, err := enum.DefaultRenderDevice()
	if err != nil {
		return fail("langkah 1 DefaultRenderDevice", err)
	}
	defer dev.Release()
	id, err := dev.ID()
	if err != nil {
		return fail("langkah 1 ID", err)
	}
	state, err := dev.State()
	if err != nil {
		return fail("langkah 1 State", err)
	}
	fmt.Printf("[1] endpoint render default: id=%s state=0x%X aktif=%v\n", id, state, state == wasapi.DeviceStateActive)
	if *step < 2 {
		return 0
	}

	// --- Langkah 2 ---
	ac, err := dev.ActivateAudioClient()
	if err != nil {
		return fail("langkah 2 ActivateAudioClient", err)
	}
	defer ac.Release()
	format, raw, err := ac.MixFormat()
	if err != nil {
		return fail("langkah 2 MixFormat", err)
	}
	defPeriod, minPeriod, err := ac.DevicePeriod()
	if err != nil {
		return fail("langkah 2 DevicePeriod", err)
	}
	fmt.Printf("[2] mix format: %s (mentah %d byte)\n", format, len(raw))
	fmt.Printf("[2] periode device: default=%v minimum=%v\n", defPeriod, minPeriod)
	if format.Kind == wasapi.KindUnknown {
		fmt.Println("[2] PERINGATAN: format tidak dikenali konverter (Tahap 2 perlu ditambah)")
	}
	if *step < 3 {
		return 0
	}

	// --- Langkah 3 ---
	if err := ac.InitializeLoopback(eventMode); err != nil {
		return fail("langkah 3 InitializeLoopback ("+*mode+")", err)
	}
	var evt windows.Handle
	if eventMode {
		evt, err = windows.CreateEvent(nil, 0, 0, nil) // auto-reset, non-signaled
		if err != nil {
			return fail("langkah 3 CreateEvent", err)
		}
		defer func() { _ = windows.CloseHandle(evt) }()
		if err := ac.SetEventHandle(evt); err != nil {
			return fail("langkah 3 SetEventHandle", err)
		}
	}
	bufFrames, err := ac.BufferSize()
	if err != nil {
		return fail("langkah 3 BufferSize", err)
	}
	cc, err := ac.CaptureClient()
	if err != nil {
		return fail("langkah 3 CaptureClient", err)
	}
	defer cc.Release()
	fmt.Printf("[3] loopback terinisialisasi (mode=%s) buffer=%d frame (%.1f ms)\n",
		*mode, bufFrames, float64(bufFrames)*1000/float64(format.SampleRate))
	if *step < 4 {
		return 0
	}

	// --- Langkah 4 ---
	var w *wavWriter
	if *wavPath != "" {
		w, err = newWAV(*wavPath, raw)
		if err != nil {
			return fail("langkah 4 buka WAV", err)
		}
		defer func() {
			if err := w.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "peringatan: tutup WAV: %v\n", err)
			}
		}()
	}
	return capture(ac, cc, format, eventMode, evt, *dur, w)
}

type stats struct {
	packets, frames              int64
	emptyPolls, timeouts         int64
	dataWithoutEvent             int64
	flagSilent, flagDisc, flagTS int64
	posJumps, posJumpFrames      int64
	gapOver50ms                  int64
	maxGap                       time.Duration
	results                      map[uint32]int64
}

func capture(ac *wasapi.AudioClient, cc *wasapi.CaptureClient, f wasapi.Format,
	eventMode bool, evt windows.Handle, dur time.Duration, w *wavWriter) int {
	st := stats{results: map[uint32]int64{}}
	if err := ac.Start(); err != nil {
		return fail("langkah 4 Start", err)
	}
	t0 := time.Now()
	cpu0 := processCPU()
	deadline := t0.Add(dur)
	nextReport := t0.Add(time.Second)
	var prev stats
	var lastPacket time.Time
	var expectedPos uint64
	havePos := false
	exit := 0

loop:
	for time.Now().Before(deadline) {
		timedOut := false
		if eventMode {
			r, err := windows.WaitForSingleObject(evt, 100)
			if err != nil {
				exit = fail("langkah 4 WaitForSingleObject", err)
				break loop
			}
			// windows.WAIT_TIMEOUT bertipe syscall.Errno di x/sys, sedangkan r bertipe
			// uint32, jadi dikonversi eksplisit (nilai tetap dari x/sys, bukan ketikan sendiri).
			if r == uint32(windows.WAIT_TIMEOUT) {
				st.timeouts++
				timedOut = true
			}
		} else {
			time.Sleep(10 * time.Millisecond)
		}

		got := int64(0)
		for {
			pkt, ok, err := cc.GetBuffer()
			if err != nil {
				exit = fail("langkah 4 GetBuffer", err)
				break loop
			}
			st.results[pkt.Result]++
			if !ok {
				if !eventMode {
					st.emptyPolls++
				}
				break
			}
			got++
			now := time.Now()
			if !lastPacket.IsZero() {
				gap := now.Sub(lastPacket)
				if gap > st.maxGap {
					st.maxGap = gap
				}
				if gap > 50*time.Millisecond {
					st.gapOver50ms++
				}
			}
			lastPacket = now
			st.packets++
			st.frames += int64(pkt.Frames)
			silent := pkt.Flags&wasapi.AudclntBufferflagsSilent != 0
			if silent {
				st.flagSilent++
			}
			if pkt.Flags&wasapi.AudclntBufferflagsDataDiscontinuity != 0 {
				st.flagDisc++
			}
			if pkt.Flags&wasapi.AudclntBufferflagsTimestampError != 0 {
				st.flagTS++
			}
			// Semantik posisi perangkat berasal dari dokumentasi, bukan header.
			if havePos && pkt.DevicePosition != expectedPos {
				st.posJumps++
				if pkt.DevicePosition > expectedPos {
					st.posJumpFrames += int64(pkt.DevicePosition - expectedPos)
				}
			}
			expectedPos = pkt.DevicePosition + uint64(pkt.Frames)
			havePos = true
			if w != nil {
				if err := w.WritePacket(pkt, silent, int(f.BlockAlign)); err != nil {
					exit = fail("langkah 4 tulis WAV", err)
					_ = cc.ReleaseBuffer(pkt.Frames)
					break loop
				}
			}
			if err := cc.ReleaseBuffer(pkt.Frames); err != nil {
				exit = fail("langkah 4 ReleaseBuffer", err)
				break loop
			}
		}
		if timedOut && got > 0 {
			st.dataWithoutEvent += got
		}

		if now := time.Now(); now.After(nextReport) {
			fmt.Printf("t=%3.0fs  paket+%d frame+%d  kosong=%d timeout=%d  silent=%d disc=%d tserr=%d  maxgap=%v\n",
				now.Sub(t0).Seconds(), st.packets-prev.packets, st.frames-prev.frames,
				st.emptyPolls, st.timeouts, st.flagSilent, st.flagDisc, st.flagTS, st.maxGap.Round(time.Millisecond))
			prev = st
			nextReport = nextReport.Add(time.Second)
		}
	}
	if err := ac.Stop(); err != nil && exit == 0 {
		exit = fail("langkah 4 Stop", err)
	}

	wall := time.Since(t0)
	cpu := processCPU() - cpu0
	expected := float64(f.SampleRate) * wall.Seconds()
	fmt.Println("---- ringkasan ----")
	fmt.Printf("lama=%v  paket=%d  frame=%d  frame_diharapkan_jam_dinding=%.0f (rasio %.3f)\n",
		wall.Round(time.Millisecond), st.packets, st.frames, expected, float64(st.frames)/expected)
	fmt.Printf("flag: SILENT=%d DATA_DISCONTINUITY=%d TIMESTAMP_ERROR=%d\n", st.flagSilent, st.flagDisc, st.flagTS)
	fmt.Printf("loncatan posisi perangkat: %d kali, total %d frame terlewat\n", st.posJumps, st.posJumpFrames)
	fmt.Printf("celah antarpaket: maks=%v, >50ms=%d kali\n", st.maxGap.Round(time.Millisecond), st.gapOver50ms)
	if eventMode {
		fmt.Printf("event timeout=%d  frame yang datang saat timeout (data tanpa event)=%d\n", st.timeouts, st.dataWithoutEvent)
	} else {
		fmt.Printf("polling kosong=%d\n", st.emptyPolls)
	}
	fmt.Printf("CPU proses: %.2f%% dari satu core, %.2f%% ternormalisasi (%d core)\n",
		100*cpu.Seconds()/wall.Seconds(), 100*cpu.Seconds()/wall.Seconds()/float64(runtime.NumCPU()), runtime.NumCPU())
	codes := make([]uint32, 0, len(st.results))
	for c := range st.results {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	fmt.Print("kode sukses GetBuffer yang muncul:")
	for _, c := range codes {
		name := wasapi.HRESULTName(c)
		if name == "" && c == 0 {
			name = "S_OK"
		}
		if name == "" {
			name = "TIDAK DIKENAL (periksa asumsi FACILITY_AUDCLNT)"
		}
		fmt.Printf("  0x%08X=%s x%d", c, name, st.results[c])
	}
	fmt.Println()
	fmt.Println("Catatan: bila tidak ada audio diputar, paket sedikit/nol adalah gejala 'loopback diam', bukan glitch.")
	return exit
}

func processCPU() time.Duration {
	h := windows.CurrentProcess()
	var c, e, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &e, &k, &u); err != nil {
		return 0
	}
	return fileTimeDur(k) + fileTimeDur(u)
}

// FILETIME dalam satuan 100 ns.
func fileTimeDur(ft windows.Filetime) time.Duration {
	return time.Duration(int64(ft.HighDateTime)<<32|int64(ft.LowDateTime)) * 100
}

// wavWriter menulis WAV dengan chunk fmt = byte mentah WAVEFORMATEX(EXTENSIBLE)
// dari GetMixFormat, jadi format asli terjaga tanpa konversi.
type wavWriter struct {
	f        *os.File
	dataOff  int64
	dataSize uint32
	fmtSize  uint32
	zeros    []byte
}

func newWAV(path string, fmtBytes []byte) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	pad := len(fmtBytes) % 2
	hdr := make([]byte, 0, 28+len(fmtBytes)+pad)
	hdr = append(hdr, "RIFF"...)
	hdr = le.AppendUint32(hdr, 0) // ukuran RIFF, diisi saat Close
	hdr = append(hdr, "WAVE"...)
	hdr = append(hdr, "fmt "...)
	hdr = le.AppendUint32(hdr, uint32(len(fmtBytes)))
	hdr = append(hdr, fmtBytes...)
	if pad == 1 {
		hdr = append(hdr, 0)
	}
	hdr = append(hdr, "data"...)
	hdr = le.AppendUint32(hdr, 0) // ukuran data, diisi saat Close
	if _, err := f.Write(hdr); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &wavWriter{f: f, dataOff: int64(len(hdr)), fmtSize: uint32(len(fmtBytes) + pad)}, nil
}

// WritePacket menulis data paket; bila silent, menulis nol (isi buffer diabaikan).
func (w *wavWriter) WritePacket(p wasapi.Packet, silent bool, blockAlign int) error {
	n := int(p.Frames) * blockAlign
	var b []byte
	if silent || len(p.Data) < n {
		if len(w.zeros) < n {
			w.zeros = make([]byte, n)
		}
		b = w.zeros[:n]
	} else {
		b = p.Data[:n]
	}
	if _, err := w.f.Write(b); err != nil {
		return err
	}
	w.dataSize += uint32(n)
	return nil
}

func (w *wavWriter) Close() error {
	le := binary.LittleEndian
	var b [4]byte
	le.PutUint32(b[:], 4+8+w.fmtSize+8+w.dataSize)
	_, err1 := w.f.WriteAt(b[:], 4)
	le.PutUint32(b[:], w.dataSize)
	_, err2 := w.f.WriteAt(b[:], w.dataOff-4)
	return errors.Join(err1, err2, w.f.Close())
}
