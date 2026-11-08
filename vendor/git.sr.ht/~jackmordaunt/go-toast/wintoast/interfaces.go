//go:build windows

// This file contains the various COM interfaces we need to call.
// Only the methods we need to call have wrappers. All methods
// that don't have a corresponding Go wrapper method are marked
// as such.
//
// The definitions are derived from:
//   - <combase.h>
//   - <windows.ui.notifications.h
//   - <NotificationActivationCallback.h>
package wintoast

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

// Runtime class names. These correspond to normal COM GUIDs, however the Windows Runtime likes
// to use string identifiers and derives the GUIDs under the hood.
const (
	CLSID_ToastNotificationManager = "Windows.UI.Notifications.ToastNotificationManager"
	CLSID_ToastNotification        = "Windows.UI.Notifications.ToastNotification"
	CLSID_XMLDocument              = "Windows.Data.Xml.Dom.XmlDocument"
)

// Interface GUIDS. These GUIDS are predefined by the Windows Runtime, identifying the various
// interfaces we want to make use of.
var (
	IID_IClassFactory                   = ole.NewGUID("{00000001-0000-0000-C000-000000000046}")
	IID_INotificationActivationCallback = ole.NewGUID("{53E31837-6600-4A81-9395-75CFFE746F94}")
	IID_ToastNotificationManager        = ole.NewGUID("{50AC103F-D235-4598-BBEF-98FE4D1A3AD4}")
	IID_ToastNotificationFactory        = ole.NewGUID("{04124B20-82C6-4229-B109-FD9ED4662B53}")
	IID_IXmlDocument                    = ole.NewGUID("{F7F3A506-1E87-42D6-BCFB-B8C809FA5494}")
	IID_IXmlDocumentIO                  = ole.NewGUID("{6CD0E74E-EE65-4489-9EBF-CA43E87BA637}")
)

// This default GUID is for our implementation.
// This was generated and should not collide with any other GUID.
var GUID_ImplNotificationActivationCallback = ole.NewGUID("{0F82E845-CB89-4039-BDBF-67CA33254C76}")

// HRESULT represents a COM return value.
type HRESULT uintptr

// IToastNotification represents a single toast notification instance.
type IToastNotification struct {
	lpVtbl *IToastNotificationVtbl
}

type IToastNotificationVtbl struct {
	ole.IInspectableVtbl
}

// IToastNotificationManager can create toast notifier objects.
type IToastNotificationManager struct {
	lpVtbl *IToastNotificationManagerVtbl

	GetContent        uintptr // not wrapped
	PutExpirationTime uintptr // not wrapped
	GetExpirationTime uintptr // not wrapped
	AddDismissed      uintptr // not wrapped
	RemoveDismissed   uintptr // not wrapped
	AddActivated      uintptr // not wrapped
	RemoveActivated   uintptr // not wrapped
	AddFailed         uintptr // not wrapped
	RemoveFailed      uintptr // not wrapped
}

type IToastNotificationManagerVtbl struct {
	ole.IInspectableVtbl

	CreateToastNotifier       uintptr // not wrapped
	CreateToastNotifierWithID uintptr
	GetTemplateContent        uintptr // not wrapped
}

func (v *IToastNotificationManager) CreateToastNotifierWithID(appID string) (ret *IToastNotifier, err error) {
	hsAppID, err := ole.NewHString(appID)
	if err != nil {
		return nil, fmt.Errorf("allocating string: %w", err)
	}
	defer ole.DeleteHString(hsAppID)
	hr, _, _ := syscall.SyscallN(
		v.lpVtbl.CreateToastNotifierWithID,
		uintptr(unsafe.Pointer(v)),
		uintptr(hsAppID),
		uintptr(unsafe.Pointer(&ret)),
	)
	if hr != ole.S_OK {
		return nil, ole.NewError(hr)
	}
	return ret, nil
}

// IToastNotifier can push notification objects to the runtime.
type IToastNotifier struct {
	lpVtbl *IToastNotifierVtbl
}

type IToastNotifierVtbl struct {
	ole.IInspectableVtbl

	Show                           uintptr
	Hide                           uintptr // not wrapped
	GetSetting                     uintptr // not wrapped
	AddToSchedule                  uintptr // not wrapped
	RemoveFromSchedule             uintptr // not wrapped
	GetScheduledToastNotifications uintptr // not wrapped
}

func (v *IToastNotifier) Show(noti *IToastNotification) (err error) {
	hr, _, _ := syscall.SyscallN(
		v.lpVtbl.Show,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(noti)),
	)
	if hr != ole.S_OK {
		return ole.NewError(hr)
	}
	return nil
}

// IToastNotificationFactory can create toast notification objects.
type IToastNotificationFactory struct {
	lpVtbl *IToastNotificationFactoryVtbl
}

type IToastNotificationFactoryVtbl struct {
	ole.IInspectableVtbl

	CreateToastNotification uintptr
}

func (v *IToastNotificationFactory) CreateToastNotification(xmlDispatch *ole.IDispatch) (ret *IToastNotification, err error) {
	hr, _, _ := syscall.SyscallN(
		v.lpVtbl.CreateToastNotification,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(xmlDispatch)),
		uintptr(unsafe.Pointer(&ret)),
	)
	if hr != ole.S_OK {
		return nil, ole.NewError(hr)
	}
	return ret, nil
}

// IXMLDocumentIO implements IO for XML documents.
type IXMLDocumentIO struct {
	lpVtbl *IXMLDocumentIOVtbl
}

type IXMLDocumentIOVtbl struct {
	ole.IInspectableVtbl

	LoadXml             uintptr
	LoadXmlWithSettings uintptr // not wrapped
	SaveToFileAsync     uintptr // not wrapped
}

func (v *IXMLDocumentIO) LoadXml(xml string) (err error) {
	hsXML, err := ole.NewHString(xml)
	if err != nil {
		return err
	}
	defer ole.DeleteHString(hsXML)
	hr, _, _ := syscall.SyscallN(
		v.lpVtbl.LoadXml,
		uintptr(unsafe.Pointer(v)),
		uintptr(hsXML),
	)
	if hr != ole.S_OK {
		return ole.NewError(hr)
	}
	return nil
}

// IClassFactory is used to build other classes. We will use this to build our implementation
// of the INotificationActivationCallback interface.
type IClassFactory struct {
	lpVtbl   *IClassFactoryVtbl
	RefCount int64
}

type IClassFactoryVtbl struct {
	ole.IUnknownVtbl
	CreateInstance uintptr // not wrapped
	LockServer     uintptr // not wrapped
}

func (v *IClassFactory) AddRef() int32 {
	count, _, _ := syscall.SyscallN(
		v.lpVtbl.AddRef,
		uintptr(unsafe.Pointer(v)),
	)
	return int32(count)
}

func (v *IClassFactory) Release() int32 {
	count, _, _ := syscall.SyscallN(
		v.lpVtbl.Release,
		uintptr(unsafe.Pointer(v)),
	)
	return int32(count)
}

// INotificationActivationCallback receives activations from toast notifications.
type INotificationActivationCallback struct {
	lpVtbl   *INotificationActivationCallbackVtbl
	RefCount int64
}

type INotificationActivationCallbackVtbl struct {
	ole.IUnknownVtbl
	Activate uintptr
}

func (v *INotificationActivationCallback) QueryInterface(riid *ole.GUID, out unsafe.Pointer) HRESULT {
	ret, _, _ := syscall.SyscallN(
		v.lpVtbl.QueryInterface,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(riid)),
		uintptr(out),
	)
	return HRESULT(ret)
}

func (v *INotificationActivationCallback) AddRef() int32 {
	count, _, _ := syscall.SyscallN(
		v.lpVtbl.AddRef,
		uintptr(unsafe.Pointer(v)),
	)
	return int32(count)
}

func (v *INotificationActivationCallback) Release() int32 {
	count, _, _ := syscall.SyscallN(
		v.lpVtbl.Release,
		uintptr(unsafe.Pointer(v)),
	)
	return int32(count)
}
