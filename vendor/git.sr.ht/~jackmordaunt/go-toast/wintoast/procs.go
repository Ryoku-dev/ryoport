//go:build windows

package wintoast

import (
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

var (
	// Define memory allocation procs. This is how we get a hold of unmanaged memory.
	kernel32   = windows.NewLazySystemDLL("kernel32.dll")
	procMalloc = kernel32.NewProc("GlobalAlloc")
	procFree   = kernel32.NewProc("GlobalFree")

	// Define procs that go-ole doesn't provide. This is how we register our Go-implemented
	// COM objects.
	modcombase              = windows.NewLazySystemDLL("combase.dll")
	procRegisterClassObject = modcombase.NewProc("CoRegisterClassObject")
)

// Allocation flags we need.
// There are more, these are just the ones we use.
// See https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalalloc
const (
	GMEM_FIXED    = 0x0000
	GMEM_ZEROINIT = 0x0040
)

// malloc allocates raw memory using the Windows kernel.
// In case of out of memory, the returned pointer will be nil.
// The memory is zeroed out to make sure we don't get garbage that looks like
// valid Go data types.
func malloc(size uintptr) unsafe.Pointer {
	hr, _, _ := procMalloc.Call(uintptr(GMEM_FIXED|GMEM_ZEROINIT), uintptr(size))
	if hr == 0 {
		return nil
	}
	return unsafe.Pointer(hr)
}

// free deallocates raw memory allocated by malloc.
func free(object unsafe.Pointer) {
	procFree.Call(uintptr(object))
}

// registerClassFactory teaches the Windows Runtime about our factory that can allocate
// instances of our ActivationCallback.
func registerClassFactory(factory *IClassFactory) error {
	// cookie is used as a handle to this class. It is used when calling CoRevokeClassObject
	// which unregisters the class. We don't need it until we plan to revoke this registration
	// for some reason.
	var cookie int64
	hr, _, _ := procRegisterClassObject.Call(
		uintptr(unsafe.Pointer(GUID_ImplNotificationActivationCallback)),
		uintptr(unsafe.Pointer(factory)),
		uintptr(ole.CLSCTX_LOCAL_SERVER),
		uintptr(1), /* REGCLS_MULTIPLEUSE */
		uintptr(unsafe.Pointer(&cookie)),
	)
	if hr != ole.S_OK {
		return ole.NewError(hr)
	}
	return nil
}
