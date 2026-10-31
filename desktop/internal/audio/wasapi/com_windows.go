//go:build windows

package wasapi

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modOle32             = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = modOle32.NewProc("CoTaskMemFree")
)

const ptrSize = unsafe.Sizeof(uintptr(0))

// InitCOM memanggil CoInitializeEx (MTA) pada thread OS yang sedang berjalan.
// Pemanggil WAJIB sudah memanggil runtime.LockOSThread() dan memakai thread
// yang sama untuk semua objek COM berikutnya. Panggil fungsi hasil kembalian
// (CoUninitialize) di thread yang sama. S_OK dan S_FALSE sama-sama sukses.
func InitCOM() (func(), error) {
	r, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	if err := check("CoInitializeEx", uint32(r)); err != nil {
		return nil, err
	}
	return func() { _, _, _ = procCoUninitialize.Call() }, nil
}

func coCreateInstance(clsid, iid *GUID, clsctx uintptr) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	r, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)),
		0,
		clsctx,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)),
	)
	if err := check("CoCreateInstance", uint32(r)); err != nil {
		return nil, err
	}
	return out, nil
}

func coTaskMemFree(p unsafe.Pointer) {
	if p != nil {
		_, _, _ = procCoTaskMemFree.Call(uintptr(p))
	}
}

// vcall memanggil metode COM pada slot vtable tertentu. Argumen pertama ke
// metode ("This") diisi dari obj. Maksimum 8 argumen tambahan.
// Objek COM dialokasikan OS (bukan heap Go), jadi uintptr(obj) aman.
// Argumen berupa alamat variabel Go ditahan hidup selama panggilan oleh
// //go:uintptrescapes.
//
//go:uintptrescapes
func vcall(obj unsafe.Pointer, slot uintptr, args ...uintptr) uint32 {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, slot*ptrSize))
	var buf [9]uintptr
	buf[0] = uintptr(obj)
	n := copy(buf[1:], args)
	r, _, _ := syscall.SyscallN(fn, buf[:n+1]...)
	return uint32(r)
}

func release(p unsafe.Pointer) {
	if p != nil {
		vcall(p, SlotRelease)
	}
}
