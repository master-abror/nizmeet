// Package wasapi membungkus WASAPI (loopback capture) tanpa CGO, lewat
// syscall ke ole32 dan pemanggilan vtable COM langsung.
//
// Paket ini TIDAK boleh mengimpor github.com/wailsapp/wails/v2 (ADR-0001).
//
// Semua konstanta, GUID, dan indeks vtable di paket ini disalin dari header
// mingw-w64 yang dilampirkan Kontributor (bukan Windows SDK), lengkap dengan
// berkas:baris sumbernya. tools/check_vtbl_slots.py memverifikasi ulang tabel
// ini terhadap header. Konstanta yang TIDAK ada di header diberi tanda
// "TIDAK TERVERIFIKASI".
package wasapi

import "fmt"

// GUID memiliki tata letak biner yang sama dengan GUID Windows (16 byte).
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// String mengembalikan bentuk kanonik huruf kecil, mis.
// "1cb9ad4c-dbfa-4c32-b178-c2f568a703b2".
func (g GUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1],
		g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

// GUID dari header lampiran. Format komentar dibaca oleh tools/check_vtbl_slots.py:
// "// berkas:baris NAMA_DI_HEADER".
var (
	CLSIDMMDeviceEnumerator = GUID{0xbcde0395, 0xe52f, 0x467c, [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}} // mmdeviceapi.h:921 CLSID_MMDeviceEnumerator
	IIDIMMDeviceEnumerator  = GUID{0xa95664d2, 0x9614, 0x4f35, [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}} // mmdeviceapi.h:646 IID_IMMDeviceEnumerator
	IIDIMMDevice            = GUID{0xd666063f, 0x1587, 0x4e43, [8]byte{0x81, 0xf1, 0xb9, 0x48, 0xe8, 0x07, 0x36, 0x3f}} // mmdeviceapi.h:303 IID_IMMDevice
	IIDIAudioClient         = GUID{0x1cb9ad4c, 0xdbfa, 0x4c32, [8]byte{0xb1, 0x78, 0xc2, 0xf5, 0x68, 0xa7, 0x03, 0xb2}} // audioclient.h:124 IID_IAudioClient
	IIDIAudioCaptureClient  = GUID{0xc8adbd64, 0xe71e, 0x48a0, [8]byte{0xa4, 0xde, 0x18, 0x5c, 0x39, 0x5c, 0xd3, 0x17}} // audioclient.h:789 IID_IAudioCaptureClient
)
