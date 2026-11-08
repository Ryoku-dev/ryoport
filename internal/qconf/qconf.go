// Package qconf reads and writes quickemu machine configs.
//
// quickemu sources these files as Bash, so Ryoport never does: it reads the
// key="value" and key=("a" "b") lines as text, keeps every other line as it
// was, and refuses values that would mean something to the shell.
package qconf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	keyRe        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	quotedItemRe = regexp.MustCompile(`"[^"]*"`)
)

// ErrUnsafe is returned for a value quickemu's shell would expand.
var ErrUnsafe = errors.New("value contains a character the config can't hold")

type Conf struct {
	lines []string
}

func Parse(data []byte) *Conf {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	c := &Conf{}
	if s != "" {
		c.lines = strings.Split(s, "\n")
	}
	return c
}

func Read(path string) (*Conf, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data), nil
}

func ValidKey(k string) bool { return keyRe.MatchString(k) }

func ValidValue(v string) bool { return !strings.ContainsAny(v, "\"`$\\\n\r") }

func (c *Conf) index(key string) int {
	prefix := key + "="
	for i, l := range c.lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

func (c *Conf) Has(key string) bool { return c.index(key) >= 0 }

// Get returns the first value for key without its quotes, or "".
func (c *Conf) Get(key string) string {
	i := c.index(key)
	if i < 0 {
		return ""
	}
	v := strings.TrimPrefix(c.lines[i], key+"=")
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return strings.Trim(v, `"`)
}

// List returns the items of a key=("a" "b") line.
func (c *Conf) List(key string) []string {
	i := c.index(key)
	if i < 0 {
		return nil
	}
	v := strings.TrimSpace(strings.TrimPrefix(c.lines[i], key+"="))
	if !strings.HasPrefix(v, "(") || !strings.HasSuffix(v, ")") {
		if s := c.Get(key); s != "" {
			return []string{s}
		}
		return nil
	}
	matches := quotedItemRe.FindAllString(v[1:len(v)-1], -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1:len(match)-1])
	}
	return out
}

func (c *Conf) replace(key, line string) {
	if i := c.index(key); i >= 0 {
		c.lines[i] = line
		for j := len(c.lines) - 1; j > i; j-- {
			if strings.HasPrefix(c.lines[j], key+"=") {
				c.lines = append(c.lines[:j], c.lines[j+1:]...)
			}
		}
		return
	}
	c.lines = append(c.lines, line)
}

func (c *Conf) Set(key, value string) error {
	if !ValidKey(key) {
		return fmt.Errorf("bad config key %q", key)
	}
	if !ValidValue(value) {
		return fmt.Errorf("%s: %w", key, ErrUnsafe)
	}
	c.replace(key, key+`="`+value+`"`)
	return nil
}

// SetList writes key=("a" "b"). An empty list removes the key.
func (c *Conf) SetList(key string, vals []string) error {
	if !ValidKey(key) {
		return fmt.Errorf("bad config key %q", key)
	}
	if len(vals) == 0 {
		c.Delete(key)
		return nil
	}
	quoted := make([]string, len(vals))
	for i, v := range vals {
		if !ValidValue(v) {
			return fmt.Errorf("%s: %w", key, ErrUnsafe)
		}
		quoted[i] = `"` + v + `"`
	}
	c.replace(key, key+"=("+strings.Join(quoted, " ")+")")
	return nil
}

func (c *Conf) Delete(key string) {
	prefix := key + "="
	out := c.lines[:0]
	for _, l := range c.lines {
		if !strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	c.lines = out
}

func (c *Conf) Bytes() []byte {
	if len(c.lines) == 0 {
		return nil
	}
	return []byte(strings.Join(c.lines, "\n") + "\n")
}

// Write replaces path atomically, keeping its mode when it already exists.
func (c *Conf) Write(path string) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(c.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
