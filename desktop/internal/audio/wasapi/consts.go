package wasapi

import (
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// Konstanta dari header. Format komentar (dibaca tools/check_vtbl_slots.py):
//   nilai  // NAMA_DI_HEADER | berkas:baris
// ---------------------------------------------------------------------------

const (
	AudclntShareModeShared    = 0 // AUDCLNT_SHAREMODE_SHARED | audiosessiontypes.h:29
	AudclntShareModeExclusive = 1 // AUDCLNT_SHAREMODE_EXCLUSIVE | audiosessiontypes.h:30
)

const (
	AudclntStreamflagsLoopback      = 0x00020000 // AUDCLNT_STREAMFLAGS_LOOPBACK | audiosessiontypes.h:45
	AudclntStreamflagsEventcallback = 0x00040000 // AUDCLNT_STREAMFLAGS_EVENTCALLBACK | audiosessiontypes.h:46
)

const (
	AudclntBufferflagsDataDiscontinuity = 0x1 // AUDCLNT_BUFFERFLAGS_DATA_DISCONTINUITY | audioclient.h:106
	AudclntBufferflagsSilent            = 0x2 // AUDCLNT_BUFFERFLAGS_SILENT | audioclient.h:107
	AudclntBufferflagsTimestampError    = 0x4 // AUDCLNT_BUFFERFLAGS_TIMESTAMP_ERROR | audioclient.h:108
)

const (
	ERender           = 0   // eRender | mmdeviceapi.h:99
	EConsole          = 0   // eConsole | mmdeviceapi.h:105
	DeviceStateActive = 0x1 // DEVICE_STATE_ACTIVE | mmdeviceapi.h:74

	WaveFormatPCM        = 1      // WAVE_FORMAT_PCM | mmreg.h:1962
	WaveFormatIEEEFloat  = 0x0003 // WAVE_FORMAT_IEEE_FLOAT | mmreg.h:1800
	WaveFormatExtensible = 0xFFFE // WAVE_FORMAT_EXTENSIBLE | mmreg.h:1943
)

// ---------------------------------------------------------------------------
// Konstanta yang TIDAK ada di keempat header lampiran (TIDAK TERVERIFIKASI).
// Nilai di bawah berasal dari pengetahuan umum tentang Windows API, bukan dari
// berkas yang dilampirkan. Verifikasi dengan header Windows/mingw lain
// (winerror.h, objbase.h/combaseapi.h, wtypes.h) sebelum dipercaya.
// ---------------------------------------------------------------------------
const (
	// FACILITY_AUDCLNT dipakai audioclient.h:1727 tetapi didefinisikan di winerror.h.
	// Menentukan nilai numerik SEMUA HRESULT AUDCLNT_E_*/AUDCLNT_S_* di bawah.
	facilityAudclnt = 0x889 // TIDAK TERVERIFIKASI

	coinitMultithreaded = 0x0  // COINIT_MULTITHREADED | TIDAK TERVERIFIKASI
	clsctxInprocServer  = 0x1  // CLSCTX_INPROC_SERVER | TIDAK TERVERIFIKASI
	clsctxAll           = 0x17 // CLSCTX_ALL | TIDAK TERVERIFIKASI

	// REFERENCE_TIME hanya di-typedef LONGLONG di header (audioclient.h); satuan
	// 100 ns berasal dari dokumentasi, bukan header. TIDAK TERVERIFIKASI.
	refTimesPerSecond = 10_000_000
	refTimeUnit       = 100 // ns per satuan REFERENCE_TIME
)

// ---------------------------------------------------------------------------
// Indeks slot vtable. Urutan diambil dari struct *Vtbl pada header (cabang
// non-C++ "typedef struct IXxxVtbl"). Komentar: "Antarmuka::Metode | berkas:baris",
// baris = baris deklarasi pointer fungsi metode di struct Vtbl.
// Slot 0..2 di setiap antarmuka adalah IUnknown (QueryInterface, AddRef, Release).
// ---------------------------------------------------------------------------
const (
	SlotQueryInterface = 0 // IUnknown, idem di semua antarmuka (tidak diperiksa baris per baris)
	SlotAddRef         = 1 // IUnknown
	SlotRelease        = 2 // IUnknown
)

const (
	SlotMMDeviceEnumeratorEnumAudioEndpoints                     = 3 // IMMDeviceEnumerator::EnumAudioEndpoints | mmdeviceapi.h:692
	SlotMMDeviceEnumeratorGetDefaultAudioEndpoint                = 4 // IMMDeviceEnumerator::GetDefaultAudioEndpoint | mmdeviceapi.h:698
	SlotMMDeviceEnumeratorGetDevice                              = 5 // IMMDeviceEnumerator::GetDevice | mmdeviceapi.h:704
	SlotMMDeviceEnumeratorRegisterEndpointNotificationCallback   = 6 // IMMDeviceEnumerator::RegisterEndpointNotificationCallback | mmdeviceapi.h:709
	SlotMMDeviceEnumeratorUnregisterEndpointNotificationCallback = 7 // IMMDeviceEnumerator::UnregisterEndpointNotificationCallback | mmdeviceapi.h:713
)

const (
	SlotMMDeviceActivate          = 3 // IMMDevice::Activate | mmdeviceapi.h:345
	SlotMMDeviceOpenPropertyStore = 4 // IMMDevice::OpenPropertyStore | mmdeviceapi.h:352
	SlotMMDeviceGetID             = 5 // IMMDevice::GetId | mmdeviceapi.h:357
	SlotMMDeviceGetState          = 6 // IMMDevice::GetState | mmdeviceapi.h:361
)

const (
	SlotAudioClientInitialize        = 3  // IAudioClient::Initialize | audioclient.h:195
	SlotAudioClientGetBufferSize     = 4  // IAudioClient::GetBufferSize | audioclient.h:204
	SlotAudioClientGetStreamLatency  = 5  // IAudioClient::GetStreamLatency | audioclient.h:208
	SlotAudioClientGetCurrentPadding = 6  // IAudioClient::GetCurrentPadding | audioclient.h:212
	SlotAudioClientIsFormatSupported = 7  // IAudioClient::IsFormatSupported | audioclient.h:216
	SlotAudioClientGetMixFormat      = 8  // IAudioClient::GetMixFormat | audioclient.h:222
	SlotAudioClientGetDevicePeriod   = 9  // IAudioClient::GetDevicePeriod | audioclient.h:226
	SlotAudioClientStart             = 10 // IAudioClient::Start | audioclient.h:231
	SlotAudioClientStop              = 11 // IAudioClient::Stop | audioclient.h:234
	SlotAudioClientReset             = 12 // IAudioClient::Reset | audioclient.h:237
	SlotAudioClientSetEventHandle    = 13 // IAudioClient::SetEventHandle | audioclient.h:240
	SlotAudioClientGetService        = 14 // IAudioClient::GetService | audioclient.h:244
)

const (
	SlotAudioCaptureClientGetBuffer         = 3 // IAudioCaptureClient::GetBuffer | audioclient.h:828
	SlotAudioCaptureClientReleaseBuffer     = 4 // IAudioCaptureClient::ReleaseBuffer | audioclient.h:836
	SlotAudioCaptureClientGetNextPacketSize = 5 // IAudioCaptureClient::GetNextPacketSize | audioclient.h:840
)

// ---------------------------------------------------------------------------
// HRESULT AUDCLNT_*. N dan Line diverifikasi terhadap audioclient.h; nilai
// numerik penuh bergantung pada facilityAudclnt (TIDAK TERVERIFIKASI).
// Format baris dibaca tools/check_vtbl_slots.py.
// ---------------------------------------------------------------------------

// HRESULTInfo menggambarkan satu makro AUDCLNT_E_* (Failure) atau AUDCLNT_S_*.
type HRESULTInfo struct {
	Name    string
	N       uint32
	Failure bool
	Line    int
}

// AudclntCodes adalah daftar lengkap makro AUDCLNT_E_*/AUDCLNT_S_* di audioclient.h:1730-1763.
var AudclntCodes = []HRESULTInfo{
	{Name: "AUDCLNT_E_NOT_INITIALIZED", N: 0x1, Failure: true, Line: 1730},
	{Name: "AUDCLNT_E_ALREADY_INITIALIZED", N: 0x2, Failure: true, Line: 1731},
	{Name: "AUDCLNT_E_WRONG_ENDPOINT_TYPE", N: 0x3, Failure: true, Line: 1732},
	{Name: "AUDCLNT_E_DEVICE_INVALIDATED", N: 0x4, Failure: true, Line: 1733},
	{Name: "AUDCLNT_E_NOT_STOPPED", N: 0x5, Failure: true, Line: 1734},
	{Name: "AUDCLNT_E_BUFFER_TOO_LARGE", N: 0x6, Failure: true, Line: 1735},
	{Name: "AUDCLNT_E_OUT_OF_ORDER", N: 0x7, Failure: true, Line: 1736},
	{Name: "AUDCLNT_E_UNSUPPORTED_FORMAT", N: 0x8, Failure: true, Line: 1737},
	{Name: "AUDCLNT_E_INVALID_SIZE", N: 0x9, Failure: true, Line: 1738},
	{Name: "AUDCLNT_E_DEVICE_IN_USE", N: 0xa, Failure: true, Line: 1739},
	{Name: "AUDCLNT_E_BUFFER_OPERATION_PENDING", N: 0xb, Failure: true, Line: 1740},
	{Name: "AUDCLNT_E_THREAD_NOT_REGISTERED", N: 0xc, Failure: true, Line: 1741},
	{Name: "AUDCLNT_E_EXCLUSIVE_MODE_NOT_ALLOWED", N: 0xe, Failure: true, Line: 1742},
	{Name: "AUDCLNT_E_ENDPOINT_CREATE_FAILED", N: 0xf, Failure: true, Line: 1743},
	{Name: "AUDCLNT_E_SERVICE_NOT_RUNNING", N: 0x10, Failure: true, Line: 1744},
	{Name: "AUDCLNT_E_EVENTHANDLE_NOT_EXPECTED", N: 0x11, Failure: true, Line: 1745},
	{Name: "AUDCLNT_E_EXCLUSIVE_MODE_ONLY", N: 0x12, Failure: true, Line: 1746},
	{Name: "AUDCLNT_E_BUFDURATION_PERIOD_NOT_EQUAL", N: 0x13, Failure: true, Line: 1747},
	{Name: "AUDCLNT_E_EVENTHANDLE_NOT_SET", N: 0x14, Failure: true, Line: 1748},
	{Name: "AUDCLNT_E_INCORRECT_BUFFER_SIZE", N: 0x15, Failure: true, Line: 1749},
	{Name: "AUDCLNT_E_BUFFER_SIZE_ERROR", N: 0x16, Failure: true, Line: 1750},
	{Name: "AUDCLNT_E_CPUUSAGE_EXCEEDED", N: 0x17, Failure: true, Line: 1751},
	{Name: "AUDCLNT_E_BUFFER_ERROR", N: 0x18, Failure: true, Line: 1752},
	{Name: "AUDCLNT_E_BUFFER_SIZE_NOT_ALIGNED", N: 0x19, Failure: true, Line: 1753},
	{Name: "AUDCLNT_E_INVALID_DEVICE_PERIOD", N: 0x20, Failure: true, Line: 1754},
	{Name: "AUDCLNT_E_INVALID_STREAM_FLAG", N: 0x21, Failure: true, Line: 1755},
	{Name: "AUDCLNT_E_ENDPOINT_OFFLOAD_NOT_CAPABLE", N: 0x22, Failure: true, Line: 1756},
	{Name: "AUDCLNT_E_OUT_OF_OFFLOAD_RESOURCES", N: 0x23, Failure: true, Line: 1757},
	{Name: "AUDCLNT_E_OFFLOAD_MODE_ONLY", N: 0x24, Failure: true, Line: 1758},
	{Name: "AUDCLNT_E_NONOFFLOAD_MODE_ONLY", N: 0x25, Failure: true, Line: 1759},
	{Name: "AUDCLNT_E_RESOURCES_INVALIDATED", N: 0x26, Failure: true, Line: 1760},
	{Name: "AUDCLNT_S_BUFFER_EMPTY", N: 0x1, Failure: false, Line: 1761},
	{Name: "AUDCLNT_S_THREAD_ALREADY_REGISTERED", N: 0x2, Failure: false, Line: 1762},
	{Name: "AUDCLNT_S_POSITION_STALLED", N: 0x3, Failure: false, Line: 1763},
}

// audclntCode menghitung MAKE_HRESULT / MAKE_SCODE (audioclient.h:1727-1728)
// untuk facilityAudclnt. Bergantung pada konstanta TIDAK TERVERIFIKASI.
func audclntCode(n uint32, failure bool) uint32 {
	c := uint32(facilityAudclnt)<<16 | n
	if failure {
		c |= 0x80000000
	}
	return c
}

// Kode yang dipakai langsung oleh logika capture.
var (
	HRBufferEmpty       = audclntCode(0x1, false) // AUDCLNT_S_BUFFER_EMPTY
	HRDeviceInvalidated = audclntCode(0x4, true)  // AUDCLNT_E_DEVICE_INVALIDATED
)

// HRESULTName mengembalikan nama makro AUDCLNT_* untuk kode, atau "" bila tidak dikenal.
func HRESULTName(code uint32) string {
	for _, e := range AudclntCodes {
		if audclntCode(e.N, e.Failure) == code {
			return e.Name
		}
	}
	return ""
}

// HRESULTError membawa HRESULT mentah dari pemanggilan COM.
type HRESULTError struct {
	Op   string
	Code uint32
}

func (e *HRESULTError) Error() string {
	if n := HRESULTName(e.Code); n != "" {
		return fmt.Sprintf("%s: HRESULT 0x%08X (%s)", e.Op, e.Code, n)
	}
	return fmt.Sprintf("%s: HRESULT 0x%08X", e.Op, e.Code)
}

// IsDeviceInvalidated melaporkan apakah err adalah AUDCLNT_E_DEVICE_INVALIDATED.
func IsDeviceInvalidated(err error) bool {
	var he *HRESULTError
	return errors.As(err, &he) && he.Code == HRDeviceInvalidated
}

// check mengubah HRESULT gagal (bit tertinggi menyala) menjadi error.
func check(op string, hr uint32) error {
	if int32(hr) < 0 {
		return &HRESULTError{Op: op, Code: hr}
	}
	return nil
}
