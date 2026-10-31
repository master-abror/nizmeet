package pcm

import (
	"fmt"
	"time"
)

// Timeline menjaga linimasa aliran keluaran berbasis jumlah sampel: aliran
// hanya bertambah di ujung (tanpa menulis ulang, jadi tanpa tumpang-tindih),
// dan selama tidak ada audio nyata, selisih terhadap jam dinding diisi sunyi
// (jadi tanpa celah).
//
// Target dihitung dari jangkar terakhir: Target(now) = emitted saat Anchor +
// (now - waktu Anchor) * laju. Tiap paket nyata memasang ulang jangkar, sehingga
// selisih laju antara jam perangkat dan jam dinding tidak menumpuk menjadi
// sunyi palsu. Semua hitungan memakai bilangan bulat (tepat, tanpa galat float).
//
// Seluruh kebijakan waktu ada di tipe ini dan di Pipeline pemakainya. Bila
// kontrak protokol (C.3) ternyata memakai timestamp eksplisit, penyesuaiannya
// terbatas di sana.
//
// Bukan untuk dipakai lintas goroutine. Anchor harus dipanggil sebelum Deficit.
type Timeline struct {
	rate          int64
	anchorTime    time.Time
	anchorEmitted int64
	emitted       int64
}

// NewTimeline membuat linimasa untuk laju keluaran rate (Hz).
func NewTimeline(rate int) (*Timeline, error) {
	if rate <= 0 {
		return nil, fmt.Errorf("pcm: laju timeline %d tidak valid", rate)
	}
	return &Timeline{rate: int64(rate)}, nil
}

// Anchor memasang jangkar pada (now, jumlah sampel yang sudah dikeluarkan).
func (t *Timeline) Anchor(now time.Time) {
	t.anchorTime = now
	t.anchorEmitted = t.emitted
}

// AddEmitted mencatat n sampel yang dikeluarkan (nyata maupun sunyi).
func (t *Timeline) AddEmitted(n int) { t.emitted += int64(n) }

// Emitted mengembalikan total sampel yang sudah dikeluarkan.
func (t *Timeline) Emitted() int64 { return t.emitted }

// Target mengembalikan jumlah sampel yang seharusnya sudah keluar pada waktu now.
func (t *Timeline) Target(now time.Time) int64 {
	ns := now.Sub(t.anchorTime).Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	const sec = int64(time.Second)
	return t.anchorEmitted + (ns/sec)*t.rate + (ns%sec)*t.rate/sec
}

// Deficit mengembalikan sampel yang perlu ditambahkan agar mencapai target (tidak pernah negatif).
func (t *Timeline) Deficit(now time.Time) int {
	if d := t.Target(now) - t.emitted; d > 0 {
		return int(d)
	}
	return 0
}

// Ahead mengembalikan seberapa jauh aliran mendahului target (negatif bila tertinggal).
func (t *Timeline) Ahead(now time.Time) time.Duration {
	return time.Duration((t.emitted - t.Target(now)) * int64(time.Second) / t.rate)
}
