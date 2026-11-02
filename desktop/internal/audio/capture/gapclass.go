package capture

// GapClass adalah hasil klasifikasi satu celah antarpaket oleh GapTracker.Observe.
type GapClass int

// Kelas celah. Klasifikasi hanya untuk celah jam dinding > 50 ms.
const (
	GapNone    GapClass = iota // tidak ada celah > 50 ms, atau paket pertama stream
	GapJitter                  // jam capture kontinu: paket terlambat, audio tidak hilang
	GapSilence                 // jam capture berlubang kira-kira sepanjang celah
	GapOther                   // di antara jitter dan hening
	GapUnknown                 // QPCPosition tidak tersedia atau berflag TIMESTAMP_ERROR
)
