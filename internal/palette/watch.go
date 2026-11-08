package palette

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/ryoku-dev/ryoport/internal/xdg"
	"golang.org/x/sys/unix"
)

const watchMask = unix.IN_CLOSE_WRITE | unix.IN_MOVED_TO | unix.IN_CREATE | unix.IN_DELETE | unix.IN_DELETE_SELF | unix.IN_MOVE_SELF

func Watch(ctx context.Context, mode func() Mode, onChange func(Palette)) error {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	wake := [2]int{}
	if err := unix.Pipe2(wake[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return err
	}
	defer unix.Close(wake[0])
	defer unix.Close(wake[1])

	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			_, _ = unix.Write(wake[1], []byte{1})
		case <-stopped:
		}
	}()

	targets := []string{xdg.RyokuPalette(), xdg.RyokuTheme(), xdg.RyokuShellSettings()}
	watches := make(map[int]string)
	if err := armWatches(fd, watches, targets); err != nil {
		return err
	}

	current := Load(mode())
	var deadline time.Time
	buffer := make([]byte, 16*1024)
	for {
		timeout := -1
		if !deadline.IsZero() {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				next := Load(mode())
				if !equal(current, next) {
					current = next
					onChange(next)
				}
				deadline = time.Time{}
				continue
			}
			timeout = int((remaining + time.Millisecond - 1) / time.Millisecond)
		}

		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}, {Fd: int32(wake[0]), Events: unix.POLLIN}}
		if _, err := unix.Poll(poll, timeout); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if poll[1].Revents != 0 {
			return nil
		}
		if poll[0].Revents == 0 {
			continue
		}

		changed, err := readEvents(fd, buffer, watches, targets)
		if err != nil {
			return err
		}
		if changed {
			if err := armWatches(fd, watches, targets); err != nil {
				return err
			}
			deadline = time.Now().Add(150 * time.Millisecond)
		}
	}
}

func equal(a, b Palette) bool {
	return a.Dark == b.Dark && a.Source == b.Source && maps.Equal(a.Roles, b.Roles)
}

func armWatches(fd int, watches map[int]string, targets []string) error {
	desired := make(map[string]bool)
	for _, target := range targets {
		desired[nearestDirectory(filepath.Dir(target))] = true
	}
	for descriptor, dir := range watches {
		if desired[dir] {
			delete(desired, dir)
			continue
		}
		_, _ = unix.InotifyRmWatch(fd, uint32(descriptor))
		delete(watches, descriptor)
	}
	for dir := range desired {
		descriptor, err := unix.InotifyAddWatch(fd, dir, watchMask)
		if err != nil {
			return err
		}
		watches[descriptor] = dir
	}
	return nil
}

func nearestDirectory(path string) string {
	for {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return parent
		}
		path = parent
	}
}

func readEvents(fd int, buffer []byte, watches map[int]string, targets []string) (bool, error) {
	changed := false
	for {
		n, err := unix.Read(fd, buffer)
		if errors.Is(err, unix.EAGAIN) {
			return changed, nil
		}
		if err != nil {
			return false, err
		}
		for offset := 0; offset+unix.SizeofInotifyEvent <= n; {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			name := ""
			if event.Len != 0 {
				raw := buffer[offset+unix.SizeofInotifyEvent : offset+unix.SizeofInotifyEvent+int(event.Len)]
				for len(raw) != 0 && raw[len(raw)-1] == 0 {
					raw = raw[:len(raw)-1]
				}
				name = string(raw)
			}
			offset += unix.SizeofInotifyEvent + int(event.Len)

			dir, ok := watches[int(event.Wd)]
			if !ok {
				continue
			}
			if event.Mask&(unix.IN_DELETE_SELF|unix.IN_MOVE_SELF|unix.IN_IGNORED) != 0 {
				delete(watches, int(event.Wd))
				changed = true
				continue
			}
			for _, target := range targets {
				relative, err := filepath.Rel(dir, target)
				if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) {
					continue
				}
				first := relative
				if split, _, ok := stringsCutPath(relative); ok {
					first = split
				}
				if name == first {
					changed = true
					break
				}
			}
		}
	}
}

func stringsCutPath(path string) (string, string, bool) {
	for i, r := range path {
		if os.IsPathSeparator(uint8(r)) {
			return path[:i], path[i+1:], true
		}
	}
	return path, "", false
}
