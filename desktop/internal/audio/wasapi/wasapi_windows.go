//go:build windows

package wasapi

import (
	"encoding/binary"
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Semua tipe di bawah TIDAK aman lintas thread: gunakan di satu goroutine
// yang terkunci ke satu thread OS (runtime.LockOSThread) setelah InitCOM.

// Enumerator membungkus IMMDeviceEnumerator.
type Enumerator struct{ p unsafe.Pointer }

// NewEnumerator membuat IMMDeviceEnumerator lewat CoCreateInstance.
func NewEnumerator() (*Enumerator, error) {
	p, err := coCreateInstance(&CLSIDMMDeviceEnumerator, &IIDIMMDeviceEnumerator, clsctxInprocServer)
	if err != nil {
		return nil, err
	}
	return &Enumerator{p: p}, nil
}

// Release melepas objek COM.
func (e *Enumerator) Release() { release(e.p); e.p = nil }

// DefaultRenderDevice mengambil endpoint render default (eRender, eConsole).
// Catatan: aplikasi seperti Zoom bisa memakai endpoint eCommunications yang
// berbeda dari eConsole; itu belum ditangani di Tahap 1.
func (e *Enumerator) DefaultRenderDevice() (*Device, error) {
	var dev unsafe.Pointer
	hr := vcall(e.p, SlotMMDeviceEnumeratorGetDefaultAudioEndpoint,
		ERender, EConsole, uintptr(unsafe.Pointer(&dev)))
	if err := check("IMMDeviceEnumerator::GetDefaultAudioEndpoint", hr); err != nil {
		return nil, err
	}
	return &Device{p: dev}, nil
}

// Device membungkus IMMDevice.
type Device struct{ p unsafe.Pointer }

// Release melepas objek COM.
func (d *Device) Release() { release(d.p); d.p = nil }

// ID mengembalikan ID endpoint (string dari IMMDevice::GetId).
func (d *Device) ID() (string, error) {
	var w unsafe.Pointer
	hr := vcall(d.p, SlotMMDeviceGetID, uintptr(unsafe.Pointer(&w)))
	if err := check("IMMDevice::GetId", hr); err != nil {
		return "", err
	}
	s := windows.UTF16PtrToString((*uint16)(w))
	coTaskMemFree(w)
	return s, nil
}

// State mengembalikan DEVICE_STATE_* (mmdeviceapi.h:74-77).
func (d *Device) State() (uint32, error) {
	var st uint32
	hr := vcall(d.p, SlotMMDeviceGetState, uintptr(unsafe.Pointer(&st)))
	if err := check("IMMDevice::GetState", hr); err != nil {
		return 0, err
	}
	return st, nil
}

// ActivateAudioClient meminta IAudioClient (bukan IAudioClient2/3) dari device.
func (d *Device) ActivateAudioClient() (*AudioClient, error) {
	var out unsafe.Pointer
	hr := vcall(d.p, SlotMMDeviceActivate,
		uintptr(unsafe.Pointer(&IIDIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&out)))
	if err := check("IMMDevice::Activate(IAudioClient)", hr); err != nil {
		return nil, err
	}
	return &AudioClient{p: out}, nil
}

// AudioClient membungkus IAudioClient.
type AudioClient struct {
	p      unsafe.Pointer
	raw    []byte // WAVEFORMATEX mentah dari GetMixFormat
	format Format
}

// Release melepas objek COM.
func (c *AudioClient) Release() { release(c.p); c.p = nil }

// MixFormat memanggil GetMixFormat, menyalin hasilnya ke memori Go, dan
// membebaskan buffer COM (CoTaskMemFree; aturan kepemilikan dari dokumentasi,
// bukan header). Mengembalikan format ter-parse dan byte mentahnya.
func (c *AudioClient) MixFormat() (Format, []byte, error) {
	var pw unsafe.Pointer
	hr := vcall(c.p, SlotAudioClientGetMixFormat, uintptr(unsafe.Pointer(&pw)))
	if err := check("IAudioClient::GetMixFormat", hr); err != nil {
		return Format{}, nil, err
	}
	defer coTaskMemFree(pw)
	head := unsafe.Slice((*byte)(pw), waveformatexSize)
	cb := int(binary.LittleEndian.Uint16(head[16:]))
	raw := make([]byte, waveformatexSize+cb)
	copy(raw, unsafe.Slice((*byte)(pw), len(raw)))
	f, err := ParseFormat(raw)
	if err != nil {
		return Format{}, nil, err
	}
	c.raw, c.format = raw, f
	return f, raw, nil
}

// DevicePeriod mengembalikan periode default dan minimum device. Konversi
// satuan 100 ns TIDAK TERVERIFIKASI (bukan dari header).
func (c *AudioClient) DevicePeriod() (def, minimum time.Duration, err error) {
	var d, m int64
	hr := vcall(c.p, SlotAudioClientGetDevicePeriod,
		uintptr(unsafe.Pointer(&d)), uintptr(unsafe.Pointer(&m)))
	if err := check("IAudioClient::GetDevicePeriod", hr); err != nil {
		return 0, 0, err
	}
	return time.Duration(d) * refTimeUnit, time.Duration(m) * refTimeUnit, nil
}

// InitializeLoopback menginisialisasi stream shared-mode dengan flag LOOPBACK
// (dan EVENTCALLBACK bila eventMode) memakai mix format dari MixFormat.
// Buffer diminta 1 detik (satuan REFERENCE_TIME TIDAK TERVERIFIKASI).
func (c *AudioClient) InitializeLoopback(eventMode bool) error {
	if len(c.raw) == 0 {
		return errors.New("InitializeLoopback: panggil MixFormat lebih dulu")
	}
	flags := uintptr(AudclntStreamflagsLoopback)
	if eventMode {
		flags |= AudclntStreamflagsEventcallback
	}
	hr := vcall(c.p, SlotAudioClientInitialize,
		AudclntShareModeShared,
		flags,
		refTimesPerSecond, // hnsBufferDuration
		0,                 // hnsPeriodicity
		uintptr(unsafe.Pointer(&c.raw[0])),
		0, // AudioSessionGuid = NULL
	)
	return check("IAudioClient::Initialize(loopback)", hr)
}

// SetEventHandle mendaftarkan event untuk mode EVENTCALLBACK (setelah Initialize, sebelum Start).
func (c *AudioClient) SetEventHandle(h windows.Handle) error {
	return check("IAudioClient::SetEventHandle", vcall(c.p, SlotAudioClientSetEventHandle, uintptr(h)))
}

// BufferSize mengembalikan ukuran buffer dalam frame.
func (c *AudioClient) BufferSize() (uint32, error) {
	var n uint32
	hr := vcall(c.p, SlotAudioClientGetBufferSize, uintptr(unsafe.Pointer(&n)))
	if err := check("IAudioClient::GetBufferSize", hr); err != nil {
		return 0, err
	}
	return n, nil
}

// Start memulai stream.
func (c *AudioClient) Start() error {
	return check("IAudioClient::Start", vcall(c.p, SlotAudioClientStart))
}

// Stop menghentikan stream.
func (c *AudioClient) Stop() error {
	return check("IAudioClient::Stop", vcall(c.p, SlotAudioClientStop))
}

// CaptureClient meminta IAudioCaptureClient lewat IAudioClient::GetService.
// Harus dipanggil setelah MixFormat dan Initialize.
func (c *AudioClient) CaptureClient() (*CaptureClient, error) {
	var out unsafe.Pointer
	hr := vcall(c.p, SlotAudioClientGetService,
		uintptr(unsafe.Pointer(&IIDIAudioCaptureClient)), uintptr(unsafe.Pointer(&out)))
	if err := check("IAudioClient::GetService(IAudioCaptureClient)", hr); err != nil {
		return nil, err
	}
	return &CaptureClient{p: out, blockAlign: int(c.format.BlockAlign)}, nil
}

// Packet adalah satu paket dari GetBuffer.
type Packet struct {
	// Data menunjuk langsung ke memori WASAPI: valid hanya sampai ReleaseBuffer.
	// Bila Flags memuat AudclntBufferflagsSilent, isi Data WAJIB diabaikan (anggap nol).
	Data           []byte
	Frames         uint32
	Flags          uint32
	DevicePosition uint64
	QPCPosition    uint64
	Result         uint32 // kode sukses mentah dari GetBuffer (untuk diagnosis)
}

// captureOut menampung parameter keluar GetBuffer di heap Go yang berumur panjang.
type captureOut struct {
	data   unsafe.Pointer
	frames uint32
	flags  uint32
	devPos uint64
	qpcPos uint64
}

// CaptureClient membungkus IAudioCaptureClient.
type CaptureClient struct {
	p          unsafe.Pointer
	blockAlign int
	out        captureOut
}

// Release melepas objek COM.
func (cc *CaptureClient) Release() { release(cc.p); cc.p = nil }

// NextPacketSize mengembalikan jumlah frame pada paket berikutnya (0 bila tidak ada).
func (cc *CaptureClient) NextPacketSize() (uint32, error) {
	var n uint32
	hr := vcall(cc.p, SlotAudioCaptureClientGetNextPacketSize, uintptr(unsafe.Pointer(&n)))
	if err := check("IAudioCaptureClient::GetNextPacketSize", hr); err != nil {
		return 0, err
	}
	return n, nil
}

// GetBuffer mengambil paket berikutnya. ok=false bila tidak ada paket
// (AUDCLNT_S_BUFFER_EMPTY atau sukses dengan 0 frame); dalam kasus itu
// ReleaseBuffer TIDAK boleh dipanggil. Bila ok=true, pemanggil harus memanggil
// ReleaseBuffer(pkt.Frames) setelah selesai memakai pkt.Data.
func (cc *CaptureClient) GetBuffer() (pkt Packet, ok bool, err error) {
	o := &cc.out
	*o = captureOut{}
	hr := vcall(cc.p, SlotAudioCaptureClientGetBuffer,
		uintptr(unsafe.Pointer(&o.data)),
		uintptr(unsafe.Pointer(&o.frames)),
		uintptr(unsafe.Pointer(&o.flags)),
		uintptr(unsafe.Pointer(&o.devPos)),
		uintptr(unsafe.Pointer(&o.qpcPos)),
	)
	if err := check("IAudioCaptureClient::GetBuffer", hr); err != nil {
		return Packet{}, false, err
	}
	if hr == HRBufferEmpty || o.frames == 0 {
		return Packet{Result: hr}, false, nil
	}
	pkt = Packet{
		Frames:         o.frames,
		Flags:          o.flags,
		DevicePosition: o.devPos,
		QPCPosition:    o.qpcPos,
		Result:         hr,
	}
	if o.data != nil {
		pkt.Data = unsafe.Slice((*byte)(o.data), int(o.frames)*cc.blockAlign)
	}
	return pkt, true, nil
}

// ReleaseBuffer melepas paket terakhir dari GetBuffer.
func (cc *CaptureClient) ReleaseBuffer(frames uint32) error {
	return check("IAudioCaptureClient::ReleaseBuffer",
		vcall(cc.p, SlotAudioCaptureClientReleaseBuffer, uintptr(frames)))
}
