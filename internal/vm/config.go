package vm

import "fmt"

var allowedConfigKeys = map[string]struct{}{
	"cpu_cores":      {},
	"ram":            {},
	"disk_size":      {},
	"display":        {},
	"guest_os":       {},
	"secureboot":     {},
	"tpm":            {},
	"iso":            {},
	"ryovm_os":       {},
	"ryovm_ssh_user": {},
	"ryovm_burn":     {},
}

// SetConfig changes one allowed scalar key on a stopped machine.
func (m *Manager) SetConfig(name, key, value string) error {
	conf, confPath, err := m.readConf(name)
	if err != nil {
		return err
	}
	if _, ok := allowedConfigKeys[key]; !ok {
		return fmt.Errorf("config key not allowed: %s", key)
	}
	_, _, pid, _, err := m.runtimePathsFor(name, conf)
	if err != nil {
		return err
	}
	if running(pid) {
		return fmt.Errorf("stop %s before changing its configuration", name)
	}
	if value == "" || value == "auto" {
		conf.Delete(key)
	} else if err := conf.Set(key, value); err != nil {
		return err
	}
	return conf.Write(confPath)
}
