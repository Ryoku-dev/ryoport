//go:build windows

// This file contains our pure-Go implementations of two COM objects that we need
// to render toast notifications: IClassFactory and INotificationActivationCallback.
//
// More specifically we allocate the C callable functions that can be used to populate
// the vtable at runtime.
//
// Unfortunately these functions have to be declared as var not const because the callbacks
// are built at runtime. They are declared globally because `syscall.NewCallback` never
// releases the memory it allocates for the functions thus causing an unsolvable memory
// leak if we were to allocate these per-notification.
package wintoast

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// Static implementations for the IClassFactory.
// syscall.NewCallback never releases its memory, so we pay that price once.
var (
	// factoryLock protects the IClassFactory reference counting.
	factoryLock sync.Mutex

	IClassFactory_AddRef = syscall.NewCallback(func(this *IClassFactory) (re uintptr) {
		factoryLock.Lock()
		defer factoryLock.Unlock()
		this.RefCount += 1
		return uintptr(this.RefCount)
	})

	IClassFactory_Release = syscall.NewCallback(func(this *IClassFactory) (re uintptr) {
		factoryLock.Lock()
		defer factoryLock.Unlock()
		this.RefCount -= 1
		if this.RefCount == 0 {
			free(unsafe.Pointer(this.lpVtbl))
			free(unsafe.Pointer(this))
		}
		return uintptr(this.RefCount)
	})

	IClassFactory_QueryInterface = syscall.NewCallback(func(this *IClassFactory, riid *ole.GUID, out unsafe.Pointer) (re uintptr) {
		if !ole.IsEqualGUID(riid, IID_IClassFactory) &&
			!ole.IsEqualGUID(riid, ole.IID_IUnknown) {
			return ole.E_NOINTERFACE
		}
		*(**IClassFactory)(out) = this
		this.AddRef()
		return uintptr(ole.S_OK)
	})

	IClassFactory_LockServer = syscall.NewCallback(func(this *IClassFactory, flock uintptr) (ret uintptr) {
		return ole.S_OK
	})

	IClassFactory_CreateInstance = syscall.NewCallback(func(this *IClassFactory, punkOuter *ole.IUnknown, riid *ole.GUID, out unsafe.Pointer) (re uintptr) {
		if punkOuter != nil {
			// Should be CLASS_E_NOAGGREGATION but ole doesn't define this.
			return ole.E_NOINTERFACE
		}
		object := newNotificationActivationCallback()
		if object == nil {
			return ole.E_OUTOFMEMORY
		}
		object.RefCount = 1
		hr := object.QueryInterface(riid, out)
		object.Release()
		return uintptr(hr)
	})
)

// Static implementations for the INotificationActivationCallback.
// syscall.NewCallback never releases its memory, so we pay that price once.
var (
	// callbackLock protects the ActivationCallback reference counting.
	callbackLock sync.Mutex

	INotificationActivationCallback_AddRef = syscall.NewCallback(func(this *INotificationActivationCallback) (re uintptr) {
		callbackLock.Lock()
		defer callbackLock.Unlock()
		this.RefCount += 1
		return uintptr(this.RefCount)
	})

	INotificationActivationCallback_Release = syscall.NewCallback(func(this *INotificationActivationCallback) (re uintptr) {
		callbackLock.Lock()
		defer callbackLock.Unlock()
		this.RefCount -= 1
		if this.RefCount == 0 {
			free(unsafe.Pointer(this.lpVtbl))
			free(unsafe.Pointer(this))
		}
		return uintptr(this.RefCount)
	})

	INotificationActivationCallback_QueryInterface = syscall.NewCallback(func(this *INotificationActivationCallback, riid *ole.GUID, out unsafe.Pointer) (re uintptr) {
		if !ole.IsEqualGUID(riid, IID_INotificationActivationCallback) &&
			!ole.IsEqualGUID(riid, ole.IID_IUnknown) {
			return ole.E_NOINTERFACE
		}
		*(**INotificationActivationCallback)(out) = this
		this.AddRef()
		return uintptr(ole.S_OK)
	})

	INotificationActivationCallback_Activate = syscall.NewCallback(func(
		this,
		appUserModelId,
		invokedArgs,
		data unsafe.Pointer,
		count uint32,
	) (ret uintptr) {
		callback(
			utf16PtrToString((*uint16)(appUserModelId)),
			utf16PtrToString((*uint16)(invokedArgs)),
			sliceUserDataFromUnsafe(data, int(count)),
		)
		return
	})
)

// newClassFactory allocates our ClassFactory that can build our NotificationActivationCallback object.
func newClassFactory() *IClassFactory {

	// Allocate the object and its vtable.

	v := (*IClassFactory)(malloc(unsafe.Sizeof(IClassFactory{})))
	if v == nil {
		return nil
	}

	v.lpVtbl = (*IClassFactoryVtbl)(malloc(unsafe.Sizeof(IClassFactoryVtbl{})))
	if v.lpVtbl == nil {
		return nil
	}

	// Provide function implementations in the Vtable.

	v.lpVtbl.AddRef = IClassFactory_AddRef
	v.lpVtbl.Release = IClassFactory_Release
	v.lpVtbl.QueryInterface = IClassFactory_QueryInterface
	v.lpVtbl.LockServer = IClassFactory_LockServer
	v.lpVtbl.CreateInstance = IClassFactory_CreateInstance

	return v
}

// newNotificationActivationCallback allocates our implementation of the INotificationActivationCallback.
func newNotificationActivationCallback() *INotificationActivationCallback {

	// Allocate the object and its vtable.

	v := (*INotificationActivationCallback)(malloc(unsafe.Sizeof(INotificationActivationCallback{})))
	if v == nil {
		return nil
	}

	v.lpVtbl = (*INotificationActivationCallbackVtbl)(malloc(unsafe.Sizeof(INotificationActivationCallbackVtbl{})))
	if v.lpVtbl == nil {
		return nil
	}

	// Provide function implementations in the vtable.

	v.lpVtbl.AddRef = INotificationActivationCallback_AddRef
	v.lpVtbl.Release = INotificationActivationCallback_Release
	v.lpVtbl.QueryInterface = INotificationActivationCallback_QueryInterface
	v.lpVtbl.Activate = INotificationActivationCallback_Activate

	return v
}
