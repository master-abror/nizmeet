package capture

import (
	"context"
	"errors"
	"time"
)

var errDefaultDeviceChanged = errors.New("capture: endpoint default berganti")

// Run menjalankan loop capture sampai ctx dibatalkan: membuka stream, memproses
// paket, mengisi sunyi saat idle, dan membuka ulang stream (dengan backoff) saat
// terjadi galat, ganti device, atau ganti format. Semua pemanggilan Opener/Source
// terjadi di goroutine pemanggil (untuk WASAPI: thread OS yang terkunci).
//
// Aturan "kuras dulu": pada tiap bangun (event atau timeout) sumber dikuras
// sampai kosong; Tick (isi sunyi) hanya dipanggil bila tidak ada paket sama sekali.
func Run(ctx context.Context, op Opener, p *Pipeline, clk Clock, cfg Config, ctr *Counters) {
	p.Begin(clk.Now())
	backoff := cfg.ReopenMin
	var gapStart time.Time // nol = sedang tidak dalam celah

	for ctx.Err() == nil {
		src, err := op.Open()
		if err == nil {
			if err = p.SetFormat(src.Format()); err != nil {
				src.Close()
			}
		}
		if err != nil {
			ctr.openFailures.Add(1)
			ctr.setErr(err)
			if gapStart.IsZero() {
				gapStart = clk.Now()
			}
			waitRetry(ctx, p, clk, cfg, backoff)
			backoff = min(backoff*2, cfg.ReopenMax)
			continue
		}
		if !gapStart.IsZero() {
			ctr.reinitGapNs.Add(int64(clk.Now().Sub(gapStart)))
			gapStart = time.Time{}
		}
		backoff = cfg.ReopenMin
		p.Resume(clk.Now())
		dev, format := src.DeviceID(), src.Format().String()
		ctr.device.Store(&dev)
		ctr.format.Store(&format)

		serr := runStream(ctx, src, op, p, clk, cfg, ctr)
		src.Close()
		if ctx.Err() != nil {
			break
		}
		ctr.reopens.Add(1)
		ctr.setErr(serr)
		gapStart = clk.Now()
	}
	p.Close()
}

func waitRetry(ctx context.Context, p *Pipeline, clk Clock, cfg Config, d time.Duration) {
	end := clk.Now().Add(d)
	for ctx.Err() == nil {
		remaining := end.Sub(clk.Now())
		if remaining <= 0 {
			return
		}
		clk.Sleep(min(cfg.EventTimeout, remaining))
		p.Tick(clk.Now())
	}
}

func runStream(ctx context.Context, src Source, op Opener, p *Pipeline, clk Clock, cfg Config, ctr *Counters) error {
	lastCheck := clk.Now()
	for ctx.Err() == nil {
		signaled, err := src.Wait(cfg.EventTimeout)
		if err != nil {
			return err
		}
		got := 0
		for {
			pk, ok, err := src.Next()
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			herr := p.HandlePacket(pk, clk.Now())
			rerr := src.Release(pk.Frames)
			if herr != nil {
				return herr
			}
			if rerr != nil {
				return rerr
			}
			got++
		}
		now := clk.Now()
		if !signaled && got > 0 {
			ctr.dataWithoutEvent.Add(uint64(got))
		}
		if got == 0 {
			p.Tick(now)
		}
		if now.Sub(lastCheck) >= cfg.DeviceCheckInterval {
			lastCheck = now
			if id, err := op.DefaultID(); err == nil && id != src.DeviceID() {
				return errDefaultDeviceChanged
			}
		}
	}
	return nil
}
