// Package keyring stores Ryoport credentials with Secret Service.
package keyring

import (
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Available reports whether the Secret Service command-line client is installed.
func Available() bool {
	_, err := exec.LookPath("secret-tool")
	return err == nil
}

// Store writes secret on stdin and identifies it by attrs.
func Store(label string, attrs map[string]string, secret string) error {
	if strings.ContainsAny(label, "\x00\r\n") {
		return errors.New("keyring label contains a control character")
	}
	args, err := attributeArgs(attrs)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("keyring attributes are empty")
	}
	cmd := exec.Command("secret-tool", append([]string{"store", "--label", label}, args...)...)
	cmd.Stdin = strings.NewReader(secret)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool store: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Lookup reads the secret identified by attrs.
func Lookup(attrs map[string]string) (string, error) {
	args, err := attributeArgs(attrs)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", errors.New("keyring attributes are empty")
	}
	out, err := exec.Command("secret-tool", append([]string{"lookup"}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("secret-tool lookup: %w", err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// Clear removes the secret identified by attrs.
func Clear(attrs map[string]string) error {
	args, err := attributeArgs(attrs)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("keyring attributes are empty")
	}
	if out, err := exec.Command("secret-tool", append([]string{"clear"}, args...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool clear: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func attributeArgs(attrs map[string]string) ([]string, error) {
	keys := make([]string, 0, len(attrs))
	for key, value := range attrs {
		if key == "" || strings.ContainsAny(key, "\x00\r\n \t") {
			return nil, fmt.Errorf("invalid keyring attribute %q", key)
		}
		if strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("invalid keyring value for %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	args := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		args = append(args, key, attrs[key])
	}
	return args, nil
}
