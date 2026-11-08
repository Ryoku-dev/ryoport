// Package console manages serial, Telnet, RDP and VNC profiles.
package console

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ryoku-dev/ryoport/internal/keyring"
	"github.com/ryoku-dev/ryoport/internal/launch"
	"github.com/ryoku-dev/ryoport/internal/xdg"
)

const (
	poolSize        = 8
	secretAttribute = "ryoport-console"
	windowClass     = "dev.ryoku.ryoport_console"
)

// Profile is one saved non-SSH console.
type Profile struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Device     string `json:"device,omitempty"`
	Baud       int    `json:"baud,omitempty"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	User       string `json:"user,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Fullscreen bool   `json:"fullscreen,omitempty"`
	Auth       string `json:"auth,omitempty"`
}

type Caps struct {
	Serial bool              `json:"serial"`
	Telnet bool              `json:"telnet"`
	RDP    bool              `json:"rdp"`
	VNC    bool              `json:"vnc"`
	Advice map[string]string `json:"advice"`
}

type Reach struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Up    bool   `json:"up"`
	RTTMs int    `json:"rttMs"`
	Error string `json:"error,omitempty"`
}

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) List() ([]Profile, error) {
	return readProfiles(), nil
}

func (m *Manager) Caps() Caps {
	return Caps{
		Serial: findBinary("picocom") != "",
		Telnet: findBinary("telnet") != "",
		RDP:    findBinary("xfreerdp3", "xfreerdp") != "",
		VNC:    findBinary("vncviewer") != "",
		Advice: map[string]string{
			"serial": installAdvice("picocom"),
			"telnet": installAdvice("inetutils"),
			"rdp":    installAdvice("freerdp"),
			"vnc":    installAdvice("tigervnc"),
		},
	}
}

func (m *Manager) Save(profile Profile) error {
	if err := validate(&profile); err != nil {
		return err
	}
	profiles := readProfiles()
	if profile.ID == "" {
		profile.ID = NewID()
	} else if !validID(profile.ID) {
		return errors.New("invalid console id")
	} else {
		for _, stored := range profiles {
			if stored.ID == profile.ID {
				profile.Auth = stored.Auth
				break
			}
		}
	}
	kept := make([]Profile, 0, len(profiles)+1)
	for _, stored := range profiles {
		if stored.ID != profile.ID {
			kept = append(kept, stored)
		}
	}
	return writeProfiles(append(kept, profile))
}

func (m *Manager) Remove(id string) error {
	if !validID(id) {
		return errors.New("invalid console id")
	}
	profiles := readProfiles()
	kept := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.ID == id {
			if keyring.Available() {
				_ = keyring.Clear(map[string]string{secretAttribute: id})
			}
			continue
		}
		kept = append(kept, profile)
	}
	return writeProfiles(kept)
}

func (m *Manager) Open(id string) error {
	profile, ok := findProfile(id)
	if !ok {
		return errors.New("no such console")
	}
	binary, err := consoleBinary(profile.Kind)
	if err != nil {
		return err
	}
	switch profile.Kind {
	case "serial":
		return launch.Terminal(serialArgv(binary, profile), windowClass, profile.Kind+": "+profile.Name)
	case "telnet":
		return launch.Terminal(telnetArgv(binary, profile), windowClass, profile.Kind+": "+profile.Name)
	case "rdp":
		if profile.Auth == "password" {
			if password, err := keyring.Lookup(map[string]string{secretAttribute: profile.ID}); err == nil && password != "" {
				reader, writer, err := os.Pipe()
				if err != nil {
					return err
				}
				go func() {
					_, _ = writer.Write([]byte(rdpArgsFile(profile, password)))
					_ = writer.Close()
				}()
				err = launch.Detached([]string{binary, "/args-from:fd:3"}, nil, []*os.File{reader})
				_ = reader.Close()
				if err != nil {
					_ = writer.Close()
				}
				return err
			}
		}
		return launch.Detached(rdpArgv(binary, profile), nil, nil)
	case "vnc":
		env := []string(nil)
		if profile.Auth == "password" {
			if password, err := keyring.Lookup(map[string]string{secretAttribute: profile.ID}); err == nil && password != "" {
				env = append(os.Environ(), "VNC_PASSWORD="+password)
				if profile.User != "" {
					env = append(env, "VNC_USERNAME="+profile.User)
				}
			}
		}
		return launch.Detached(vncArgv(binary, profile), env, nil)
	}
	return errors.New("unknown console kind")
}

func (m *Manager) Probe(ctx context.Context, id string) Reach {
	profile, ok := findProfile(id)
	if !ok {
		return Reach{ID: id, RTTMs: -1, Error: "no such console"}
	}
	return probe(ctx, profile)
}

func (m *Manager) ProbeAll(ctx context.Context) []Reach {
	profiles := readProfiles()
	results := make([]Reach, len(profiles))
	sem := make(chan struct{}, poolSize)
	var wg sync.WaitGroup
	for i, profile := range profiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				results[i] = probe(ctx, profile)
			case <-ctx.Done():
				results[i] = Reach{ID: profile.ID, Kind: profile.Kind, RTTMs: -1, Error: ctx.Err().Error()}
			}
		}()
	}
	wg.Wait()
	return results
}

func (m *Manager) SetPassword(id, password string) error {
	profile, ok := findProfile(id)
	if !ok {
		return errors.New("no such console")
	}
	if password == "" {
		return errors.New("no password")
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return errors.New("password contains a control character")
	}
	if !keyring.Available() {
		return errors.New("secret-tool not found: install libsecret to save passwords")
	}
	if err := keyring.Store("ryoport console: "+profile.Name, map[string]string{secretAttribute: profile.ID}, password); err != nil {
		return err
	}
	profiles := readProfiles()
	for i := range profiles {
		if profiles[i].ID == id {
			profiles[i].Auth = "password"
			return writeProfiles(profiles)
		}
	}
	return errors.New("no such console")
}

func (m *Manager) ClearPassword(id string) error {
	if !validID(id) {
		return errors.New("invalid console id")
	}
	if keyring.Available() {
		_ = keyring.Clear(map[string]string{secretAttribute: id})
	}
	profiles := readProfiles()
	for i := range profiles {
		if profiles[i].ID == id {
			profiles[i].Auth = ""
			return writeProfiles(profiles)
		}
	}
	return nil
}

func NewID() string {
	var data [4]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}

func readProfiles() []Profile {
	data, err := os.ReadFile(xdg.ConsolesFile())
	if err != nil {
		return []Profile{}
	}
	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil || profiles == nil {
		return []Profile{}
	}
	return profiles
}

func writeProfiles(profiles []Profile) error {
	if profiles == nil {
		profiles = []Profile{}
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(xdg.ConsolesFile(), append(data, '\n'), 0o600)
}

func findProfile(id string) (Profile, bool) {
	for _, profile := range readProfiles() {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

func validate(profile *Profile) error {
	switch profile.Kind {
	case "serial", "telnet", "rdp", "vnc":
	default:
		return errors.New("kind must be serial, telnet, rdp, or vnc")
	}
	if strings.ContainsAny(profile.Name, "\x00\r\n") {
		return errors.New("name must not contain control characters")
	}
	for _, value := range []string{profile.Device, profile.Address, profile.User, profile.Domain} {
		if strings.ContainsAny(value, "\x00\r\n") {
			return errors.New("fields must not contain control characters")
		}
		if strings.HasPrefix(value, "-") {
			return errors.New("fields must not start with '-'")
		}
	}
	switch profile.Kind {
	case "serial":
		if !strings.HasPrefix(profile.Device, "/dev/") || strings.ContainsAny(profile.Device, " \t") {
			return errors.New("serial device must be a /dev/ path without spaces")
		}
		if profile.Baud < 30 || profile.Baud > 4000000 {
			return errors.New("baud must be between 30 and 4000000")
		}
	case "telnet":
		if !validHost(profile.Address) {
			return errors.New("telnet address must be a bare host (letters, digits, . _ -)")
		}
		if profile.Port < 1 || profile.Port > 65535 {
			profile.Port = 23
		}
	case "rdp":
		if !validHost(profile.Address) {
			return errors.New("RDP address must be a bare host (letters, digits, . _ -)")
		}
		if profile.Port < 1 || profile.Port > 65535 {
			profile.Port = 3389
		}
	case "vnc":
		if !validHost(profile.Address) {
			return errors.New("VNC address must be a bare host (letters, digits, . _ -)")
		}
		if profile.Port < 1 || profile.Port > 65535 {
			profile.Port = 5900
		}
	}
	return nil
}

func consoleBinary(kind string) (string, error) {
	switch kind {
	case "serial":
		if binary := findBinary("picocom"); binary != "" {
			return binary, nil
		}
		return "", fmt.Errorf("no serial client found: %s", installAdvice("picocom"))
	case "telnet":
		if binary := findBinary("telnet"); binary != "" {
			return binary, nil
		}
		return "", fmt.Errorf("no telnet client found: %s", installAdvice("inetutils"))
	case "rdp":
		if binary := findBinary("xfreerdp3", "xfreerdp"); binary != "" {
			return binary, nil
		}
		return "", fmt.Errorf("no RDP client found: %s", installAdvice("freerdp"))
	case "vnc":
		if binary := findBinary("vncviewer"); binary != "" {
			return binary, nil
		}
		return "", fmt.Errorf("no VNC client found: %s", installAdvice("tigervnc"))
	default:
		return "", fmt.Errorf("unknown console kind %q", kind)
	}
}

func installAdvice(name string) string {
	output, err := exec.Command("ryoku-host", "pkg", "advice", name).Output()
	if err == nil {
		if advice := strings.TrimSpace(string(output)); advice != "" {
			return advice
		}
	}
	return "install " + name
}

func findBinary(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func serialArgv(binary string, profile Profile) []string {
	return []string{binary, "--baud", strconv.Itoa(profile.Baud), "--flow", "n", profile.Device}
}

func telnetArgv(binary string, profile Profile) []string {
	return []string{binary, profile.Address, strconv.Itoa(profile.Port)}
}

func rdpArgs(profile Profile) []string {
	args := []string{
		"/v:" + net.JoinHostPort(profile.Address, strconv.Itoa(profile.Port)),
		"/cert:tofu",
		"/wm-class:" + windowClass,
		"/t:ryoport: " + profile.Name,
	}
	if profile.User != "" {
		args = append(args, "/u:"+profile.User)
	}
	if profile.Domain != "" {
		args = append(args, "/d:"+profile.Domain)
	}
	if profile.Fullscreen {
		args = append(args, "/f")
	} else {
		args = append(args, "/size:1280x800")
	}
	return args
}

func rdpArgv(binary string, profile Profile) []string {
	return append([]string{binary}, rdpArgs(profile)...)
}

func rdpArgsFile(profile Profile, password string) string {
	return strings.Join(append(rdpArgs(profile), "/p:"+password), "\n") + "\n"
}

func vncArgv(binary string, profile Profile) []string {
	args := []string{binary, profile.Address + "::" + strconv.Itoa(profile.Port)}
	if profile.Fullscreen {
		args = append(args, "-FullScreen")
	}
	return args
}

func probe(ctx context.Context, profile Profile) Reach {
	result := Reach{ID: profile.ID, Kind: profile.Kind, RTTMs: -1}
	if profile.Kind == "serial" {
		info, err := os.Stat(profile.Device)
		switch {
		case err != nil:
			result.Error = "device not present: " + profile.Device
		case info.Mode()&os.ModeDevice == 0:
			result.Error = "not a device node: " + profile.Device
		case syscall.Access(profile.Device, 0o6) != nil:
			result.Error = "no permission on " + profile.Device + " (add yourself to the operator group)"
		default:
			result.Up = true
			result.RTTMs = 0
		}
		return result
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	connection, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(profile.Address, strconv.Itoa(profile.Port)))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	connection.Close()
	result.Up = true
	result.RTTMs = int(time.Since(start).Milliseconds())
	return result
}

func validHost(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case char == '.' || char == '-' || char == '_':
		default:
			return false
		}
	}
	return true
}

func validID(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "\x00\r\n \t/") {
		return false
	}
	return true
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := xdg.EnsureDir(dir); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "."+filepath.Base(path)+".bak"), old, mode)
	}
	temp, err := os.CreateTemp(dir, ".ryoport-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(name)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	remove = false
	return nil
}
