package vm

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	usbIDPattern   = regexp.MustCompile(`^[0-9a-f]{4}:[0-9a-f]{4}$`)
	lsusbPattern   = regexp.MustCompile(`ID ([0-9a-f]{4}:[0-9a-f]{4}) ?(.*)$`)
	forwardPattern = regexp.MustCompile(`^[0-9]+:[0-9]+$`)
)

// USBDevice is one host USB device and its next-boot assignment state.
type USBDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Assigned bool   `json:"assigned"`
}

// USB lists assignable host USB devices.
func (m *Manager) USB(ctx context.Context, name string) ([]USBDevice, error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return nil, err
	}
	assigned := make(map[string]struct{})
	for _, id := range conf.List("usb_devices") {
		assigned[id] = struct{}{}
	}
	output, err := exec.CommandContext(ctx, "lsusb").Output()
	if err != nil {
		return []USBDevice{}, nil
	}
	devices := make([]USBDevice, 0)
	for line := range strings.SplitSeq(string(output), "\n") {
		match := lsusbPattern.FindStringSubmatch(line)
		if match == nil || strings.Contains(match[2], "root hub") {
			continue
		}
		label := strings.TrimSpace(match[2])
		if label == "" {
			label = "USB device"
		}
		_, on := assigned[match[1]]
		devices = append(devices, USBDevice{ID: match[1], Name: label, Assigned: on})
	}
	return devices, nil
}

// SetUSB changes one next-boot USB assignment on a stopped machine.
func (m *Manager) SetUSB(name, id string, on bool) error {
	if !usbIDPattern.MatchString(id) {
		return fmt.Errorf("bad device id: %s", id)
	}
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	_, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before changing USB devices", name)
	}
	current := conf.List("usb_devices")
	updated := make([]string, 0, len(current)+1)
	for _, currentID := range current {
		if currentID != id {
			updated = append(updated, currentID)
		}
	}
	if on {
		updated = append(updated, id)
	}
	if err := conf.SetList("usb_devices", updated); err != nil {
		return err
	}
	return conf.Write(confPath)
}

// PortForward is one host-to-guest TCP port mapping.
type PortForward struct {
	Host  string `json:"host"`
	Guest string `json:"guest"`
}

// PortForwards returns the configured next-boot mappings.
func (m *Manager) PortForwards(name string) ([]PortForward, error) {
	conf, _, err := m.readConf(name)
	if err != nil {
		return nil, err
	}
	values := conf.List("port_forwards")
	forwards := make([]PortForward, 0, len(values))
	for _, value := range values {
		host, guest, ok := strings.Cut(value, ":")
		if !ok {
			guest = value
		}
		if index := strings.LastIndexByte(value, ':'); index >= 0 {
			guest = value[index+1:]
		}
		forwards = append(forwards, PortForward{Host: host, Guest: guest})
	}
	return forwards, nil
}

// AddPortForward adds a mapping, replacing any mapping on the same host port.
func (m *Manager) AddPortForward(name, spec string) error {
	host, _, err := validateForward(spec)
	if err != nil {
		return err
	}
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	_, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before changing port forwards", name)
	}
	values := conf.List("port_forwards")
	updated := make([]string, 0, len(values)+1)
	for _, value := range values {
		currentHost, _, _ := strings.Cut(value, ":")
		if currentHost != host {
			updated = append(updated, value)
		}
	}
	updated = append(updated, spec)
	if err := conf.SetList("port_forwards", updated); err != nil {
		return err
	}
	return conf.Write(confPath)
}

// RemovePortForward removes an exact mapping from a stopped machine.
func (m *Manager) RemovePortForward(name, spec string) error {
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	_, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before changing port forwards", name)
	}
	values := conf.List("port_forwards")
	updated := make([]string, 0, len(values))
	for _, value := range values {
		if value != spec {
			updated = append(updated, value)
		}
	}
	if err := conf.SetList("port_forwards", updated); err != nil {
		return err
	}
	return conf.Write(confPath)
}

func validateForward(spec string) (host, guest string, err error) {
	if !forwardPattern.MatchString(spec) {
		return "", "", fmt.Errorf("bad forward '%s' (want hostport:guestport)", spec)
	}
	host, guest, _ = strings.Cut(spec, ":")
	hostPort, _ := strconv.Atoi(host)
	guestPort, _ := strconv.Atoi(guest)
	if hostPort < 1 || hostPort > 65535 || guestPort < 1 || guestPort > 65535 {
		return "", "", fmt.Errorf("port out of range in '%s'", spec)
	}
	return host, guest, nil
}
