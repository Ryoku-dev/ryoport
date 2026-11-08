package app

import (
	"errors"
	"fmt"
	"io"

	"gioui.org/x/explorer"
)

type fileChooser struct {
	explorer *explorer.Explorer
	post     func(func())
}

func newFileChooser(fileExplorer *explorer.Explorer, post func(func())) *fileChooser {
	return &fileChooser{explorer: fileExplorer, post: post}
}

func (f *fileChooser) ChooseFile(extensions []string, done func(path string, err error)) {
	go func() {
		file, err := f.explorer.ChooseFile(extensions...)
		if err != nil {
			f.deliver(done, "", err)
			return
		}
		path, pathErr := chosenPath(file)
		if file != nil {
			_ = file.Close()
		}
		f.deliver(done, path, pathErr)
	}()
}

func (f *fileChooser) deliver(done func(string, error), path string, err error) {
	if done == nil {
		return
	}
	f.post(func() { done(path, err) })
}

func chosenPath(file io.ReadCloser) (string, error) {
	if named, ok := file.(interface{ Name() string }); ok && named.Name() != "" {
		return named.Name(), nil
	}
	if located, ok := file.(interface{ URI() string }); ok && located.URI() != "" {
		return located.URI(), nil
	}
	if file == nil {
		return "", errors.New("the file chooser returned no file")
	}
	return "", fmt.Errorf("the file chooser returned %T without a usable path", file)
}
