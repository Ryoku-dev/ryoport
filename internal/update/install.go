package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ryoku-dev/ryoport/internal/xdg"
)

func (client *Client) applyPacman(ctx context.Context, packagePath, latest string) error {
	out, err := client.commandRunner().Output(ctx, "/usr/bin/pacman", "-Qp", packagePath)
	if err != nil {
		return fmt.Errorf("inspect downloaded package: %w", err)
	}
	expected := "ryoport " + latest + "-1"
	if strings.TrimSpace(string(out)) != expected {
		return fmt.Errorf("downloaded package identity does not match %q", expected)
	}
	if err := client.commandRunner().Run(context.WithoutCancel(ctx), "/usr/bin/pkexec", "/usr/bin/pacman", "-U", "--noconfirm", packagePath); err != nil {
		return fmt.Errorf("install update: %w", err)
	}
	return nil
}

func (client *Client) applyTarball(ctx context.Context, archivePath, latest, updateStaging string) error {
	extracted := filepath.Join(updateStaging, "tree")
	if err := os.Mkdir(extracted, 0o700); err != nil {
		return err
	}
	files, err := extractTarball(archivePath, extracted)
	if err != nil {
		return err
	}
	if err := validateTarballLayout(extracted, files); err != nil {
		return err
	}
	out, err := client.commandRunner().Output(ctx, filepath.Join(extracted, "bin", "ryoport"), "version")
	if err != nil {
		return fmt.Errorf("run staged Ryoport: %w", err)
	}
	if !reportsVersion(string(out), latest) {
		return fmt.Errorf("staged Ryoport reports %q, expected %s", strings.TrimSpace(string(out)), latest)
	}
	return installTree(extracted, files, latest)
}

func extractTarball(archivePath, destination string) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	seen := map[string]bool{}
	var files []string
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		name, err := cleanArchivePath(header.Name)
		if err != nil {
			return nil, err
		}
		if name == "" {
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("release archive repeats %s", name)
		}
		seen[name] = true
		target := filepath.Join(destination, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxPackage {
				return nil, fmt.Errorf("invalid archived file size for %s", name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
			mode := os.FileMode(0o644)
			if name == "bin/ryoport" {
				mode = 0o755
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return nil, err
			}
			_, copyErr := io.CopyN(out, reader, header.Size)
			closeErr := out.Close()
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			files = append(files, name)
		case tar.TypeSymlink:
			if !validAlias(name, header.Linkname) {
				return nil, fmt.Errorf("unsafe release symlink %s -> %s", name, header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
			if err := os.Symlink("ryoport", target); err != nil {
				return nil, err
			}
			files = append(files, name)
		case tar.TypeLink:
			if !validAlias(name, header.Linkname) {
				return nil, fmt.Errorf("unsafe release hard link %s -> %s", name, header.Linkname)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return nil, err
			}
			if err := os.Link(filepath.Join(destination, "bin", "ryoport"), target); err != nil {
				return nil, err
			}
			files = append(files, name)
		default:
			return nil, fmt.Errorf("unsupported release archive entry %s", name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func validateTarballLayout(root string, files []string) error {
	required := []string{
		"bin/ryoport",
		"bin/ryossh",
		"bin/ryovm",
		"bin/ryovm-fetch",
		"bin/ryovm-mon",
		"share/applications/dev.ryoku.ryoport.desktop",
		"share/icons/hicolor/scalable/apps/dev.ryoku.ryoport.svg",
		"share/metainfo/dev.ryoku.ryoport.metainfo.xml",
	}
	set := make(map[string]bool, len(files))
	license := false
	for _, name := range files {
		set[name] = true
		if strings.HasPrefix(name, "share/licenses/ryoport/") && name != "share/licenses/ryoport/" {
			license = true
			continue
		}
		// allowed but not required, so a release from before the marker still installs
		if name == markerFile {
			continue
		}
		allowed := false
		for _, expected := range required {
			if name == expected {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("unexpected file in release archive: %s", name)
		}
	}
	for _, name := range required {
		if !set[name] {
			return fmt.Errorf("release archive is missing %s", name)
		}
	}
	if !license {
		return errors.New("release archive has no license files")
	}
	info, err := os.Stat(filepath.Join(root, "bin", "ryoport"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("release archive has no executable bin/ryoport")
	}
	return nil
}

func installTree(root string, files []string, latest string) error {
	local := filepath.Join(xdg.Home(), ".local")
	type preparedFile struct {
		rel  string
		dest string
		tmp  string
		old  string
	}
	prepared := make([]preparedFile, 0, len(files))
	cleanupPrepared := func() {
		for _, file := range prepared {
			os.Remove(file.tmp)
		}
	}
	defer cleanupPrepared()
	for _, rel := range files {
		source := filepath.Join(root, filepath.FromSlash(rel))
		dest := filepath.Join(local, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		tmp, err := prepareInstalledFile(source, filepath.Dir(dest))
		if err != nil {
			return err
		}
		prepared = append(prepared, preparedFile{rel: rel, dest: dest, tmp: tmp, old: dest + ".old"})
	}
	installed := 0
	rollback := func() {
		for i := installed - 1; i >= 0; i-- {
			file := prepared[i]
			os.Remove(file.dest)
			if _, err := os.Lstat(file.old); err == nil {
				os.Rename(file.old, file.dest)
			}
		}
	}
	for i, file := range prepared {
		if err := os.Remove(file.old); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollback()
			return err
		}
		if _, err := os.Lstat(file.dest); err == nil {
			if err := os.Rename(file.dest, file.old); err != nil {
				rollback()
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			rollback()
			return err
		}
		if err := os.Rename(file.tmp, file.dest); err != nil {
			if _, oldErr := os.Lstat(file.old); oldErr == nil {
				os.Rename(file.old, file.dest)
			}
			rollback()
			return err
		}
		prepared[i].tmp = ""
		installed++
	}
	marker := installedMarker{Version: latest, Files: append([]string(nil), files...)}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		rollback()
		return err
	}
	data = append(data, '\n')
	if err := atomicWriteFile(markerPath(), data, 0o600); err != nil {
		rollback()
		return err
	}
	for _, file := range prepared {
		if err := os.Remove(file.old); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func prepareInstalledFile(source, destinationDir string) (string, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(destinationDir, ".ryoport-install-*")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return "", err
		}
		if err := os.Symlink(target, name); err != nil {
			return "", err
		}
		return name, nil
	}
	in, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer in.Close()
	mode := os.FileMode(0o644)
	if info.Mode().Perm()&0o111 != 0 {
		mode = 0o755
	}
	out, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(name)
		return "", err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(name)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".installed-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func cleanArchivePath(name string) (string, error) {
	name = filepath.ToSlash(strings.TrimPrefix(name, "./"))
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe path in release archive: %s", name)
	}
	return clean, nil
}

func validAlias(name, target string) bool {
	switch name {
	case "bin/ryovm", "bin/ryossh", "bin/ryovm-mon", "bin/ryovm-fetch":
		return target == "ryoport" || target == "bin/ryoport"
	default:
		return false
	}
}

func reportsVersion(output, version string) bool {
	fields := strings.Fields(strings.TrimSpace(output))
	return len(fields) > 0 && (fields[len(fields)-1] == version || fields[len(fields)-1] == "v"+version)
}
