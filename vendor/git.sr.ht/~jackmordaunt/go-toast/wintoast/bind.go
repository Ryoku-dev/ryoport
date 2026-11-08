// Package wintoast provides a pure-Go implementation of toast notifications on Windows.
package wintoast

import "errors"

// AppData describes the various application metadata that can be used with Windows
// toast notifications.
type AppData struct {
	// AppID of the application. This should be a pretty name as it will be displayed in
	// the notification.
	AppID string

	// GUID to use for this application. Strongly recommended to supply a your own,
	// however a default one will be used otherwise.
	GUID string

	// ActivationExe is the full path to an executable that Windows will invoke when
	// the application is not running. This can be used to cold-start the application.
	// Windows will provide an extra flag, so the named executable must be able to handle extra flags.
	ActivationExe string // optional

	// IconPath is the full path to an icon that Windows will display for the notification.
	IconPath string // optional

	// IconBackgroundColor is a hex encoded color code that Windows will display as the background
	// for the named icon.
	IconBackgroundColor string // optional
}

// UserData contains Key:Value pairs generated within the notification, based
// on the XML content of the notification. Specifically, all inputs within
// the XML will generate a corresponding UserData struct.
type UserData struct {
	Key   string
	Value string
}

// Callback is a function that gets invoked when the notification is activated.
type Callback func(appUserModelId string, invokedArgs string, userData []UserData)

// SetAppData teaches the Windows Runtime about our application and establishes the activation GUID
// so Windows will know how to invoke us back.
func SetAppData(data AppData) (err error) {
	return setAppData(data)
}

// SetActivationCallback establishes the callback `cb` to be invoked when
// the toast notification is activated. This callback instance should handle
// being activated from any available toast notification.
func SetActivationCallback(cb Callback) {
	callback = cb
}

// Push a notification described by the XML to the Windows Runtime.
//
// App data should be set first via a call to SetAppData before calling
// this function.
//
// If the powershell fallback is engaged, activation callbacks will not
// work as expected and the COM error will still be returned.
func Push(xml string, op ...option) error {
	var opts options
	for _, opt := range op {
		opt(&opts)
	}
	if opts.PowershellPreferred {
		return pushPowershell(xml)
	}
	if err := pushCOM(xml); err != nil {
		if opts.PowershellFallback {
			return errors.Join(err, pushPowershell(xml))
		}
		return err
	}
	return nil
}

type options struct {
	PowershellFallback  bool
	PowershellPreferred bool
}

type option func(*options)

// PreferPowershell indicates to use the powershell method by default.
// COM will not be used.
func PreferPowershell(opt *options) {
	opt.PowershellPreferred = true
}

// PowershellFallback specifies to use the powershell method as a fallback
// if the COM api fails.
func PowershellFallback(opt *options) {
	opt.PowershellFallback = true
}

// callback is the global callback reference that is invoked by Activate.
//
// NOTE(jfm): synchronize access to this?
var callback Callback = func(model, args string, data []UserData) {}
