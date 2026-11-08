//go:build windows

package wintoast

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast/tmpl"
	"github.com/go-ole/go-ole"
)

func pushPowershell(xml string) error {
	f, err := os.CreateTemp("", "*.ps1")
	if err != nil {
		return fmt.Errorf("creating temporary script file: %w", err)
	}

	defer func() { err = errors.Join(err, os.Remove(f.Name())) }()

	// This BOM ensures we can support non-ascii characters in the toast content.
	bomUtf8 := []byte{0xef, 0xbb, 0xbf}
	if _, err := f.Write(bomUtf8); err != nil {
		return fmt.Errorf("writing utf8 byte marker: %w", err)
	}

	if err := buildPowershell(xml, f); err != nil {
		return fmt.Errorf("generating powershell script: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing script file: %w", err)
	}

	cmd := exec.Command("PowerShell", "-ExecutionPolicy", "Bypass", "-File", f.Name())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("executing powershell: %q: %w", string(out), err)
	}

	return nil
}

func buildPowershell(xml string, w io.Writer) error {
	type scriptData struct {
		AppID string
		XML   string
	}
	return tmpl.ScriptTemplate.Execute(w, scriptData{AppID: appData.AppID, XML: xml})
}

func pushCOM(xml string) error {
	if err := initialize(); err != nil {
		return err
	}

	// 1. allocate ClassFactory implementation.
	// 2. register ClassFactory implementation (provides our ActivationCallback to the runtime)
	// 3. load noti manager (statics impl)
	// 4. load noti factory
	// 5. create xml
	// 6. create xmlIO
	// 7. load xml
	// 8. create noti

	classFactory := newClassFactory()
	if classFactory == nil {
		return fmt.Errorf("could not allocate class factory")
	}

	if err := registerClassFactory(classFactory); err != nil {
		return fmt.Errorf("registering class factory: %w", err)
	}

	noti, err := newNotiFromXml(xml)
	if err != nil {
		return fmt.Errorf("building notification: %w", err)
	}

	notifier, err := newNotifier(appData.AppID)
	if err != nil {
		return fmt.Errorf("building notifier: %w", err)
	}

	if err := notifier.Show(noti); err != nil {
		return fmt.Errorf("showing notification: %w", err)
	}

	return nil
}

func setAppData(data AppData) (err error) {
	appDataMu.Lock()
	defer appDataMu.Unlock()

	// Early out if we have already set this data.
	//
	// In the case the data is empty, we don't want to overrite
	// all of the registry entries to empty.
	//
	// This allows the caller to either globally set the app data
	// or provide it per notification.
	if appData == data || data.AppID == "" {
		return nil
	}

	if data.GUID != "" {
		GUID_ImplNotificationActivationCallback = ole.NewGUID(data.GUID)
	}

	// Keep a copy of the saved data for later.
	defer func() {
		if err == nil {
			appData = data
		}
	}()

	if err := setAppDataFunc(data); err != nil {
		return err
	}

	return nil
}

var initLock sync.Mutex
var didInitialize bool

// initialize attempts to initialize the Windows Runtime.
// Each invocation will retry RoInitialize until a successful initialization
// is achieved. Once initialized, we avoid invoking RoInitialize since subsequent
// reinitialization generates errors.
func initialize() (err error) {
	initLock.Lock()
	defer initLock.Unlock()

	if didInitialize {
		return nil
	}

	if err := ole.RoInitialize(1); err != nil {
		return fmt.Errorf("RoInitialize: %w", err)
	}

	didInitialize = true

	return nil
}

// newNotifier builds an IToastNotifier instance using the given appID.
func newNotifier(appID string) (*IToastNotifier, error) {
	managerObject, err := ole.RoGetActivationFactory(CLSID_ToastNotificationManager, IID_ToastNotificationManager)
	if err != nil {
		return nil, fmt.Errorf("getting activation factory: %w", err)
	}

	// Get access to the manager vtable.
	manager := (*IToastNotificationManager)(unsafe.Pointer(managerObject))

	notifier, err := manager.CreateToastNotifierWithID(appID)
	if err != nil {
		return nil, fmt.Errorf("creating toast notifier: %w", err)
	}

	return notifier, nil
}

// newNotiFromXml builds an IToastNotification instance from the given xml content.
func newNotiFromXml(xml string) (*IToastNotification, error) {
	factoryObject, err := ole.RoGetActivationFactory(CLSID_ToastNotification, IID_ToastNotificationFactory)
	if err != nil {
		return nil, fmt.Errorf("getting activation factory: %w", err)
	}

	// Get access to the factory vtable.
	factory := (*IToastNotificationFactory)(unsafe.Pointer(factoryObject))

	xmlDoc, err := loadXML(xml)
	if err != nil {
		return nil, fmt.Errorf("loading xml: %w", err)
	}

	noti, err := factory.CreateToastNotification(xmlDoc)
	if err != nil {
		return nil, fmt.Errorf("creating toast notification: %w", err)
	}

	return noti, nil
}

// loadXML allocates an XML document object (returned as IDispatch because we don't care
// about representing it's vtable).
func loadXML(xml string) (*ole.IDispatch, error) {
	xmlDocObject, err := ole.RoActivateInstance(CLSID_XMLDocument)
	if err != nil {
		return nil, fmt.Errorf("RoActivateInstance: %w", err)
	}

	xmlDoc, err := xmlDocObject.QueryInterface(IID_IXmlDocument)
	if err != nil {
		return nil, fmt.Errorf("querying IID_IXmlDocument: %w", err)
	}

	xmlDocIO, err := xmlDoc.QueryInterface(IID_IXmlDocumentIO)
	if err != nil {
		return nil, fmt.Errorf("querying interface IID_IXmlDocumentIO: %w", err)
	}

	// Get access to the IO vtable.
	xmlIO := (*IXMLDocumentIO)(unsafe.Pointer(xmlDocIO))

	if err := xmlIO.LoadXml(xml); err != nil {
		return nil, fmt.Errorf("IXmlDocumentIO.LoadXml: %w", err)
	}

	return xmlDoc, nil
}

// sliceUserDataFromUnsafe builds a slice of UserData out of an unsafe pointer.
func sliceUserDataFromUnsafe(ptr unsafe.Pointer, count int) []UserData {

	// Layout mirrors the memory layout of the C struct that contains this data.
	// I'm not sure if there's special alignment or packing - though I don't notice
	// anything in the definition to indicate as such.
	type layout struct {
		Key   unsafe.Pointer
		Value unsafe.Pointer
	}

	// Create a new slice with the appropriate length
	out := make([]UserData, count)

	// Create a slice with the unsafe data layout.
	tmp := unsafe.Slice((*layout)(ptr), count)

	// Convert the unsafe layout to safe strings.
	for ii, it := range tmp {
		out[ii] = UserData{
			Key:   utf16PtrToString((*uint16)(it.Key)),
			Value: utf16PtrToString((*uint16)(it.Value)),
		}
	}

	return out
}

// utf16PtrToString builds a string out of a utf16 null terminated byte sequence.
//
// Copied from package syscall.
func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	// Find NUL terminator.
	end := unsafe.Pointer(p)
	n := 0
	for *(*uint16)(end) != 0 {
		end = unsafe.Pointer(uintptr(end) + unsafe.Sizeof(*p))
		n++
	}
	// Turn *uint16 into []uint16.
	s := unsafe.Slice(p, n)
	// Decode []uint16 into string.
	return string(utf16.Decode(s))
}
