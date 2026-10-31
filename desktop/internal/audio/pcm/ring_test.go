package pcm

import (
	"runtime"
	"testing"
	"time"
)

func TestRingWrapAround(t *testing.T) {
	r, err := NewRing(8)
	if err != nil {
		t.Fatal(err)
	}
	if n := r.Write([]int16{1, 2, 3, 4, 5}); n != 5 {
		t.Fatalf("Write = %d", n)
	}
	if r.Len() != 5 || r.Free() != 3 || r.Cap() != 8 {
		t.Fatalf("Len=%d Free=%d Cap=%d", r.Len(), r.Free(), r.Cap())
	}
	buf := make([]int16, 3)
	if n := r.Read(buf); n != 3 || buf[0] != 1 || buf[2] != 3 {
		t.Fatalf("Read = %d %v", n, buf)
	}
	if n := r.Write([]int16{6, 7, 8, 9, 10}); n != 5 { // membungkus melewati ujung buffer
		t.Fatalf("Write (wrap) = %d", n)
	}
	out := make([]int16, 16)
	n := r.Read(out)
	want := []int16{4, 5, 6, 7, 8, 9, 10}
	if n != len(want) {
		t.Fatalf("Read = %d, want %d", n, len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("[%d] = %d, want %d", i, out[i], want[i])
		}
	}
	if r.Dropped() != 0 {
		t.Errorf("Dropped = %d", r.Dropped())
	}
}

func TestRingOverrunDropsNewest(t *testing.T) {
	r, _ := NewRing(8)
	r.Write([]int16{1, 2, 3, 4, 5, 6})
	if n := r.Write([]int16{7, 8, 9, 10, 11}); n != 2 {
		t.Fatalf("Write saat hampir penuh = %d, want 2", n)
	}
	if r.Dropped() != 3 {
		t.Errorf("Dropped = %d, want 3", r.Dropped())
	}
	out := make([]int16, 16)
	n := r.Read(out)
	if n != 8 {
		t.Fatalf("Read = %d, want 8", n)
	}
	for i := 0; i < 8; i++ {
		if out[i] != int16(i+1) {
			t.Errorf("[%d] = %d, want %d", i, out[i], i+1)
		}
	}
	if n := r.Write(nil); n != 0 {
		t.Errorf("Write(nil) = %d", n)
	}
}

func TestNewRingError(t *testing.T) {
	if _, err := NewRing(0); err == nil {
		t.Error("ingin error untuk kapasitas 0")
	}
}

// Uji beban satu produsen + satu konsumen. Produsen menunggu ruang (tanpa
// kehilangan), konsumen memeriksa urutan. Tidak memakai race detector.
func TestRingConcurrentSPSC(t *testing.T) {
	const total = 400000
	r, _ := NewRing(1000)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		seq := 0
		chunk := make([]int16, 0, 300)
		step := 1
		for seq < total {
			n := min(step, total-seq)
			step = step%300 + 1
			chunk = chunk[:0]
			for i := 0; i < n; i++ {
				chunk = append(chunk, int16((seq+i)%30011))
			}
			for r.Free() < n {
				select {
				case <-stop:
					return
				default:
					runtime.Gosched()
				}
			}
			if w := r.Write(chunk); w != n {
				return // tidak boleh terjadi: ruang sudah dipastikan
			}
			seq += n
		}
	}()

	buf := make([]int16, 137)
	got := 0
	deadline := time.Now().Add(30 * time.Second)
	for got < total {
		n := r.Read(buf)
		for i := 0; i < n; i++ {
			if want := int16((got + i) % 30011); buf[i] != want {
				close(stop)
				t.Fatalf("urutan rusak di sampel %d: got %d, want %d", got+i, buf[i], want)
			}
		}
		got += n
		if n == 0 {
			if time.Now().After(deadline) {
				close(stop)
				t.Fatalf("timeout: baru %d dari %d sampel", got, total)
			}
			runtime.Gosched()
		}
	}
	<-done
	if r.Dropped() != 0 {
		t.Errorf("Dropped = %d, want 0", r.Dropped())
	}
}
