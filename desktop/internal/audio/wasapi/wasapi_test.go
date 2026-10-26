package wasapi

import (
	"encoding/binary"
	"testing"
)

// Cross-check: string GUID dari MIDL_INTERFACE di header (representasi kedua di
// berkas yang sama) harus sama dengan literal numerik DEFINE_GUID yang disalin.
// Berkas:baris string: audioclient.h:126 dan :791, mmdeviceapi.h:305 dan :648.
func TestGUIDStringsMatchMIDLInterface(t *testing.T) {
	cases := []struct {
		name string
		got  GUID
		want string
	}{
		{"IAudioClient", IIDIAudioClient, "1cb9ad4c-dbfa-4c32-b178-c2f568a703b2"},
		{"IAudioCaptureClient", IIDIAudioCaptureClient, "c8adbd64-e71e-48a0-a4de-185c395cd317"},
		{"IMMDevice", IIDIMMDevice, "d666063f-1587-4e43-81f1-b948e807363f"},
		{"IMMDeviceEnumerator", IIDIMMDeviceEnumerator, "a95664d2-9614-4f35-a746-de8db63617e6"},
	}
	for _, c := range cases {
		if got := c.got.String(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func buildWFX(tag, ch uint16, rate uint32, bits uint16, extra []byte) []byte {
	b := make([]byte, 18, 18+len(extra))
	le := binary.LittleEndian
	le.PutUint16(b[0:], tag)
	le.PutUint16(b[2:], ch)
	le.PutUint32(b[4:], rate)
	block := ch * bits / 8
	le.PutUint32(b[8:], rate*uint32(block))
	le.PutUint16(b[12:], block)
	le.PutUint16(b[14:], bits)
	le.PutUint16(b[16:], uint16(len(extra)))
	return append(b, extra...)
}

func extensibleExtra(valid uint16, mask uint32, subTag uint16) []byte {
	e := make([]byte, 22)
	binary.LittleEndian.PutUint16(e[0:], valid)
	binary.LittleEndian.PutUint32(e[2:], mask)
	// SubFormat GUID standar: {subTag-0000-0010-8000-00aa00389b71} (mmreg.h:2005-2013)
	binary.LittleEndian.PutUint32(e[6:], uint32(subTag))
	binary.LittleEndian.PutUint16(e[10:], 0)
	binary.LittleEndian.PutUint16(e[12:], 0x0010)
	copy(e[14:], []byte{0x80, 0x00, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71})
	return e
}

func TestParseFormat(t *testing.T) {
	t.Run("pcm16 stereo", func(t *testing.T) {
		f, err := ParseFormat(buildWFX(WaveFormatPCM, 2, 44100, 16, nil))
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != KindInt16 || f.Channels != 2 || f.SampleRate != 44100 || f.BlockAlign != 4 || f.Extensible {
			t.Errorf("tidak sesuai: %+v", f)
		}
	})
	t.Run("extensible float32", func(t *testing.T) {
		raw := buildWFX(WaveFormatExtensible, 2, 48000, 32, extensibleExtra(32, 0x3, WaveFormatIEEEFloat))
		if len(raw) != waveformatextensibleSize {
			t.Fatalf("panjang %d, want %d", len(raw), waveformatextensibleSize)
		}
		f, err := ParseFormat(raw)
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != KindFloat32 || !f.Extensible || f.ChannelMask != 0x3 || f.ValidBits != 32 ||
			f.EffectiveTag != WaveFormatIEEEFloat || f.BlockAlign != 8 {
			t.Errorf("tidak sesuai: %+v", f)
		}
	})
	t.Run("extensible pcm24 container 24", func(t *testing.T) {
		raw := buildWFX(WaveFormatExtensible, 2, 96000, 24, extensibleExtra(24, 0x3, WaveFormatPCM))
		f, err := ParseFormat(raw)
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != KindInt24 {
			t.Errorf("kind %v, want int24", f.Kind)
		}
	})
	t.Run("extensible terpotong", func(t *testing.T) {
		raw := buildWFX(WaveFormatExtensible, 2, 48000, 32, extensibleExtra(32, 0x3, WaveFormatIEEEFloat))
		if _, err := ParseFormat(raw[:30]); err == nil {
			t.Error("ingin error untuk buffer terpotong")
		}
	})
	t.Run("subformat tidak dikenal", func(t *testing.T) {
		e := extensibleExtra(32, 0x3, WaveFormatIEEEFloat)
		e[14] = 0x00 // rusak ekor GUID standar
		f, err := ParseFormat(buildWFX(WaveFormatExtensible, 2, 48000, 32, e))
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != KindUnknown || f.EffectiveTag != 0 {
			t.Errorf("ingin unknown, dapat %+v", f)
		}
	})
	t.Run("terlalu pendek", func(t *testing.T) {
		if _, err := ParseFormat(make([]byte, 10)); err == nil {
			t.Error("ingin error")
		}
	})
}

// Konsistensi internal tabel HRESULT (BUKAN verifikasi nilai facility, yang
// tidak ada di header lampiran).
func TestAudclntCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range AudclntCodes {
		if seen[e.Name] {
			t.Errorf("nama ganda: %s", e.Name)
		}
		seen[e.Name] = true
		code := audclntCode(e.N, e.Failure)
		if (int32(code) < 0) != e.Failure {
			t.Errorf("%s: bit severity tidak sesuai", e.Name)
		}
		if HRESULTName(code) != e.Name {
			t.Errorf("%s: pencarian nama gagal", e.Name)
		}
	}
	if HRESULTName(HRBufferEmpty) != "AUDCLNT_S_BUFFER_EMPTY" {
		t.Error("HRBufferEmpty tidak cocok dengan tabel")
	}
	if HRESULTName(HRDeviceInvalidated) != "AUDCLNT_E_DEVICE_INVALIDATED" {
		t.Error("HRDeviceInvalidated tidak cocok dengan tabel")
	}
	if !IsDeviceInvalidated(&HRESULTError{Op: "x", Code: HRDeviceInvalidated}) {
		t.Error("IsDeviceInvalidated gagal")
	}
}
