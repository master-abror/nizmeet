package capture

import "time"

// catchUp mengisi sunyi untuk celah panjang yang tidak sempat diisi Tick: stream
// yang dibuka ulang (Open memblokir), proses yang macet, dan sejenisnya. Dipanggil
// dari HandlePacket SEBELUM sampel paket ini dikeluarkan, sehingga sunyi berada
// di depan audio nyata (tanpa tumpang-tindih).
//
// pending adalah jumlah sampel keluaran milik paket ini. Pad = Deficit(now) - pending,
// karena paket itu sendiri mewakili ~durasi paket terakhir sebelum now. Pad hanya
// dikeluarkan bila >= IdleThreshold (dalam sampel): celah lebih pendek dianggap
// jitter pengiriman (audio terlambat, bukan hilang) dan tidak diisi. Setelah
// Tick mengisi idle, selisihnya kecil sehingga tidak ada pad ganda.
//
// Paket nyata pertama sejak Start tidak pernah memicu pad: latensi pembukaan
// awal bukan celah audio.
func (p *Pipeline) catchUp(now time.Time, pending int) {
	if !p.everPacket {
		return
	}
	minPad := int(int64(p.cfg.IdleThreshold) * int64(p.cfg.OutRate) / int64(time.Second))
	pad := p.tl.Deficit(now) - pending
	if pad < minPad {
		return
	}
	if limit := int(int64(p.cfg.MaxCatchUp) * int64(p.cfg.OutRate) / int64(time.Second)); pad > limit {
		pad = limit
		p.resyncs.Add(1)
	}
	p.emitSilence(pad, &p.silenceGap)
}

// Resume dipanggil Run setiap kali stream berhasil dibuka. Selama belum ada
// paket nyata, linimasa dimulai ulang dari sekarang (waktu pembukaan awal
// yang lama tidak menjadi sunyi di depan aliran). Setelah ada paket nyata,
// tidak melakukan apa pun: celah sesudahnya diisi oleh catchUp.
func (p *Pipeline) Resume(now time.Time) {
	if !p.everPacket {
		p.Begin(now)
	}
}

// SilenceIdle mengembalikan sunyi pengisi idle (hening tanpa paket, diisi Tick).
func (p *Pipeline) SilenceIdle() int64 { return p.silence.Load() }

// SilenceGap mengembalikan sunyi pengisi celah (buka ulang stream, macet).
func (p *Pipeline) SilenceGap() int64 { return p.silenceGap.Load() }

// FramePad mengembalikan nol pelengkap frame yang ditambahkan saat Close.
func (p *Pipeline) FramePad() int64 { return p.framePad.Load() }

// Gaps mengembalikan statistik celah antarpaket.
func (p *Pipeline) Gaps() GapSnapshot { return p.gaps.Snapshot() }

// GapIntervalMax mengembalikan celah terbesar sejak pemanggilan sebelumnya (untuk pencatat per detik).
func (p *Pipeline) GapIntervalMax() time.Duration { return p.gaps.TakeIntervalMax() }
