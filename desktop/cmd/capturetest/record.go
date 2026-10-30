//go:build windows

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/master-abror/nizmeet/desktop/internal/audio/capture"
)

// Ukuran memori working set lewat K32GetProcessMemoryInfo (kernel32). Struktur
// dan fungsi ini dari pengetahuan umum Windows API, BUKAN dari header lampiran:
// TIDAK TERVERIFIKASI. Bila gagal, nilainya 0 dan hanya mengurangi informasi.
var (
	modKernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessMemoryInfo = modKernel32.NewProc("K32GetProcessMemoryInfo")
)

// processMemoryCounters meniru PROCESS_MEMORY_COUNTERS (8 SIZE_T setelah dua DWORD).
type processMemoryCounters struct {
	cb                 uint32
	_                  uint32
	peakWorkingSetSize uintptr
	workingSetSize     uintptr
	_                  [6]uintptr
}

func workingSet() (cur, peak uint64) {
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	r, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	if r == 0 {
		return 0, 0
	}
	return uint64(c.workingSetSize), uint64(c.peakWorkingSetSize)
}

// recorder mencatat ukuran per detik ke CSV (opsional) dan merangkum akhir uji.
type recorder struct {
	file       *os.File
	w          *csv.Writer
	outRate    int
	takeGapMax func() time.Duration

	started  bool
	prevWall time.Time
	prevCPU  time.Duration

	cpuCore, cpuNorm []float64
	peakHeap         uint64
	peakSys          uint64
	firstOut         time.Duration
	haveFirst        bool
}

var csvHeader = []string{
	"t_s", "out_samples", "silence_idle", "silence_gap", "idle",
	"glitch_pct", "lost_s", "active_s", "pos_jumps", "dropped_frames",
	"reopens", "reinit_gap_ms", "packets", "silent_packets", "disc_flags", "disc_unexplained",
	"data_without_event", "gap_max_ms_interval", "gap_max_ms_total",
	"gaps_jitter", "gaps_silence", "gaps_other", "gaps_unknown",
	"cpu_core_pct", "cpu_norm_pct", "heap_mb", "sys_mb", "ws_mb", "goroutines", "ring_len",
}

func newRecorder(path string, outRate int, takeGapMax func() time.Duration) (*recorder, error) {
	r := &recorder{outRate: outRate, takeGapMax: takeGapMax}
	if path == "" {
		return r, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	r.file = f
	r.w = csv.NewWriter(f)
	if err := r.w.Write(csvHeader); err != nil {
		_ = f.Close()
		return nil, err
	}
	r.w.Flush()
	return r, nil
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1f", float64(d)/float64(time.Millisecond)) }

func mb(b uint64) float64 { return float64(b) / (1 << 20) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Sample dipanggil sekali per detik.
func (r *recorder) Sample(elapsed time.Duration, info capture.Info, ringLen int) {
	now := time.Now()
	cpu := processCPU()
	var coreP, normP float64
	if r.started {
		if dt := now.Sub(r.prevWall).Seconds(); dt > 0 {
			coreP = 100 * (cpu - r.prevCPU).Seconds() / dt
			normP = coreP / float64(runtime.NumCPU())
			r.cpuCore = append(r.cpuCore, coreP)
			r.cpuNorm = append(r.cpuNorm, normP)
		}
	}
	r.started, r.prevWall, r.prevCPU = true, now, cpu

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	r.peakHeap = max(r.peakHeap, m.HeapAlloc)
	r.peakSys = max(r.peakSys, m.Sys)
	ws, _ := workingSet()
	if !r.haveFirst && info.SamplesOut > 0 {
		r.haveFirst, r.firstOut = true, elapsed
	}
	gapMax := r.takeGapMax()
	s := info.Stats

	if r.w != nil {
		row := []string{
			fmt.Sprintf("%.1f", elapsed.Seconds()),
			fmt.Sprint(info.SamplesOut), fmt.Sprint(info.SilenceIdle), fmt.Sprint(info.SilenceGap), fmt.Sprint(b2i(info.Idle)),
			fmt.Sprintf("%.4f", s.GlitchRatio()*100), fmt.Sprintf("%.3f", s.LostSeconds), fmt.Sprintf("%.3f", s.ActiveSeconds),
			fmt.Sprint(s.PositionJumps), fmt.Sprint(info.DroppedFrames),
			fmt.Sprint(info.Reopens), fmt.Sprint(info.ReinitGap.Milliseconds()),
			fmt.Sprint(s.Packets), fmt.Sprint(s.SilentPackets), fmt.Sprint(s.DiscontinuityFlags), fmt.Sprint(s.DiscontinuityUnexplained),
			fmt.Sprint(info.DataWithoutEvent), ms(gapMax), ms(info.Gaps.Max),
			fmt.Sprint(info.Gaps.Jitter), fmt.Sprint(info.Gaps.Silence), fmt.Sprint(info.Gaps.Other), fmt.Sprint(info.Gaps.Unknown),
			fmt.Sprintf("%.2f", coreP), fmt.Sprintf("%.2f", normP),
			fmt.Sprintf("%.1f", mb(m.HeapAlloc)), fmt.Sprintf("%.1f", mb(m.Sys)), fmt.Sprintf("%.1f", mb(ws)),
			fmt.Sprint(runtime.NumGoroutine()), fmt.Sprint(ringLen),
		}
		_ = r.w.Write(row) // galat tulis akan muncul di Close lewat w.Error()
		r.w.Flush()
	}
	fmt.Printf("t=%4.0fs keluar=%d idle=%d sunyi(idle/celah)=%d/%d glitch=%.3f%% cpu=%.1f%%/%.1f%% celah-maks=%sms heap=%.1fMB ws=%.1fMB ring=%d\n",
		elapsed.Seconds(), info.SamplesOut, b2i(info.Idle), info.SilenceIdle, info.SilenceGap,
		s.GlitchRatio()*100, coreP, normP, ms(gapMax), mb(m.HeapAlloc), mb(ws), ringLen)
}

func (r *recorder) Close() {
	if r.w != nil {
		r.w.Flush()
		if err := r.w.Error(); err != nil {
			fmt.Fprintf(os.Stderr, "peringatan: tulis CSV: %v\n", err)
		}
	}
	if r.file != nil {
		_ = r.file.Close()
	}
}

func stats3(x []float64) (mean, p95, maxv float64) {
	if len(x) == 0 {
		return 0, 0, 0
	}
	s := append([]float64(nil), x...)
	sort.Float64s(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	return sum / float64(len(s)), s[int(0.95*float64(len(s)-1))], s[len(s)-1]
}

func verdict(ok bool) string {
	if ok {
		return "LULUS"
	}
	return "GAGAL"
}

// Summary mencetak ringkasan akhir untuk acuan Acceptance Criteria 3.1.
func (r *recorder) Summary(wall time.Duration, info capture.Info) {
	fmt.Println("---- ukuran Tahap 4 ----")
	g := info.Stats.GlitchRatio() * 100
	fmt.Printf("AC glitch < 1%%: %.4f%% (hilang %.3fs dari %.3fs aktif) -> %s\n",
		g, info.Stats.LostSeconds, info.Stats.ActiveSeconds, verdict(g < 1))

	if len(r.cpuCore) > 0 {
		mc, pc, xc := stats3(r.cpuCore)
		mn, pn, xn := stats3(r.cpuNorm)
		fmt.Printf("CPU per-core: rata-rata %.2f%% p95 %.2f%% maks %.2f%% | ternormalisasi: rata-rata %.2f%% p95 %.2f%% maks %.2f%%\n",
			mc, pc, xc, mn, pn, xn)
		fmt.Printf("AC CPU < 5%% rata-rata: per-core %.2f%% -> %s | ternormalisasi %.2f%% -> %s\n",
			mc, verdict(mc < 5), mn, verdict(mn < 5))
	}
	_, peakWS := workingSet()
	fmt.Printf("memori: heap Go puncak %.1f MB, Sys Go puncak %.1f MB, working set puncak %.1f MB (silang-cek dengan Task Manager)\n",
		mb(r.peakHeap), mb(r.peakSys), mb(peakWS))

	if r.haveFirst {
		active := wall - r.firstOut
		if active > 0 {
			fmt.Printf("rasio keluaran/jam-dinding sejak keluaran pertama (~%v): %.4f\n",
				r.firstOut.Round(time.Second), float64(info.SamplesOut)/(float64(r.outRate)*active.Seconds()))
		}
	}
	fmt.Printf("sunyi: idle=%d celah=%d pad-frame-akhir=%d (sampel @ %d Hz)\n",
		info.SilenceIdle, info.SilenceGap, info.FramePad, r.outRate)

	gp := info.Gaps
	fmt.Printf("celah antarpaket: maks=%v; >50ms: %d (jitter=%d hening=%d lain=%d tak-diketahui=%d)\n",
		gp.Max.Round(time.Millisecond), gp.Count, gp.Jitter, gp.Silence, gp.Other, gp.Unknown)
	fmt.Print("histogram celah:")
	for i, lbl := range capture.GapBucketLabels {
		fmt.Printf("  %s=%d", lbl, gp.Hist[i])
	}
	fmt.Println()
	if gp.Jitter > 0 {
		fmt.Printf("jitter terbesar (jam dinding): %v\n", gp.MaxJitter.Round(time.Millisecond))
	}
	if gp.Silence > 0 {
		fmt.Printf("lubang jam capture terbesar saat hening: %v\n", gp.MaxSilenceHole.Round(time.Millisecond))
	}
	switch q := gp.QPCRatio(); {
	case q == 0:
		fmt.Println("QPC: tidak ada data (QPCPosition = 0 atau TIMESTAMP_ERROR); pembeda jitter/hening tidak bisa dipakai")
	case q < 0.98 || q > 1.02:
		fmt.Printf("QPC: rasio rentang QPC/jam dinding = %.4f -> ASUMSI SATUAN 100 ns KEMUNGKINAN SALAH; abaikan klasifikasi jitter/hening\n", q)
	default:
		fmt.Printf("QPC: rasio rentang QPC/jam dinding = %.4f -> satuan 100 ns konsisten dengan jam dinding\n", q)
	}
}
