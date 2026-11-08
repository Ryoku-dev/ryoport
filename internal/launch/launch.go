// Package launch starts detached desktop helpers without passing through a shell.
package launch

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Terminal opens argv in a terminal window identified by class and title.
func Terminal(argv []string, class, title string) error {
	term, known, err := ResolveTerminal()
	if err != nil {
		return err
	}
	return TerminalResolved(term, known, argv, class, title)
}

// TerminalResolved opens argv using a terminal returned by ResolveTerminal.
func TerminalResolved(term string, known bool, argv []string, class, title string) error {
	if err := validArgv(argv); err != nil {
		return err
	}
	if err := validArgv([]string{term}); err != nil {
		return err
	}
	return Detached(terminalArgv(term, known, argv, class, title), nil, nil)
}

func terminalArgv(term string, known bool, argv []string, class, title string) []string {
	hold := `"$@" || { printf "\n── press enter to close ──\n"; read _; }`
	program := append([]string{"sh", "-c", hold, "_"}, argv...)
	if !known {
		return append([]string{term}, program...)
	}
	switch filepath.Base(term) {
	case "kitty":
		return append([]string{term, "--title", title, "--class=" + class}, program...)
	case "ghostty":
		return append([]string{term, "--class=" + class, "-e"}, program...)
	case "foot", "footclient":
		return append([]string{term, "--title", title, "--app-id=" + class}, program...)
	case "alacritty":
		return append([]string{term, "--class=" + class, "-e"}, program...)
	case "wezterm":
		return append([]string{term, "start", "--"}, program...)
	default:
		return append([]string{term, "-e"}, program...)
	}
}

// Detached starts argv in its own session with no inherited standard streams.
func Detached(argv []string, env []string, files []*os.File) error {
	if err := validArgv(argv); err != nil {
		return err
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = devnull
	cmd.Stdout = devnull
	cmd.Stderr = devnull
	cmd.ExtraFiles = files
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Start(); err != nil {
		devnull.Close()
		return fmt.Errorf("launch %s: %w", argv[0], err)
	}
	devnull.Close()
	go cmd.Wait()
	return nil
}

// OpenURL opens a URL with the desktop handler.
func OpenURL(rawURL string) error {
	if strings.ContainsRune(rawURL, 0) || strings.HasPrefix(rawURL, "-") {
		return errors.New("invalid URL")
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || u.Scheme == "" {
		return errors.New("invalid URL")
	}
	return Detached([]string{"xdg-open", rawURL}, nil, nil)
}

// OpenPath opens a local path with the desktop handler.
func OpenPath(path string) error {
	if path == "" || strings.ContainsRune(path, 0) {
		return errors.New("invalid path")
	}
	if strings.HasPrefix(path, "-") {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		path = absolute
	}
	return Detached([]string{"xdg-open", path}, nil, nil)
}

// OpenSFTP opens an SFTP location in Nautilus, falling back to xdg-open.
func OpenSFTP(user, host string, port int) error {
	if !validEndpointPart(user, true, false) || !validEndpointPart(host, false, true) {
		return errors.New("invalid SFTP endpoint")
	}
	if port == 0 {
		port = 22
	}
	if port < 1 || port > 65535 {
		return errors.New("invalid SFTP port")
	}
	u := &url.URL{Scheme: "sftp", Host: host, Path: "/"}
	if strings.Contains(host, ":") {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	} else if port != 22 {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	if user != "" {
		u.User = url.User(user)
	}
	name := "xdg-open"
	if _, err := exec.LookPath("nautilus"); err == nil {
		name = "nautilus"
	}
	return Detached([]string{name, u.String()}, nil, nil)
}

// ResolveTerminal returns the executable and whether its argument syntax is known.
func ResolveTerminal() (path string, known bool, err error) {
	if configured := os.Getenv("TERMINAL"); configured != "" {
		if strings.ContainsAny(configured, "\x00\r\n") {
			return "", false, errors.New("invalid TERMINAL")
		}
		return configured, true, nil
	}
	if path, err := exec.LookPath("xdg-terminal-exec"); err == nil {
		return path, false, nil
	}
	for _, name := range []string{"kitty", "ghostty", "foot", "footclient", "alacritty", "wezterm", "xterm"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, true, nil
		}
	}
	return "", false, errors.New("no terminal found")
}

func validArgv(argv []string) error {
	if len(argv) == 0 || argv[0] == "" {
		return errors.New("empty command")
	}
	for _, arg := range argv {
		if strings.ContainsRune(arg, 0) {
			return errors.New("command argument contains NUL")
		}
	}
	return nil
}

func validEndpointPart(value string, emptyOK, allowColon bool) bool {
	if value == "" {
		return emptyOK
	}
	if strings.HasPrefix(value, "-") {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case char == '.' || char == '-' || char == '_':
		case allowColon && char == ':':
		default:
			return false
		}
	}
	return true
}
