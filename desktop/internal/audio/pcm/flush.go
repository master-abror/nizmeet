package pcm

// Flush mengeluarkan sisa keluaran yang tertahan oleh latensi filter lalu
// me-Reset state. Caranya: memasukkan nol sepanjang riwayat filter, sehingga
// ekor audio terakhir (sekitar Latency()) ikut keluar sebelum hening.
//
// Dipanggil saat masuk idle atau saat mengganti format. Tanpa Flush, ~2 ms audio
// terakhir sebelum hening tertahan di filter dan bocor ke awal audio berikutnya.
// Pada mode passthrough tidak ada yang tertahan, dst dikembalikan apa adanya.
func (r *Resampler) Flush(dst []float32) []float32 {
	if r.passthrough {
		return dst
	}
	dst = r.Process(dst, make([]float32, r.k))
	r.Reset()
	return dst
}
