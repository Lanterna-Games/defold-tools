package lsp

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"time"
)

type overlay struct {
	disk    fs.FS
	unsaved map[string][]byte
}

func (o overlay) Open(name string) (fs.File, error) {
	if content, isOpenInTheEditor := o.unsaved[name]; isOpenInTheEditor {
		return &unsavedFile{name: path.Base(name), reader: bytes.NewReader(content), size: int64(len(content))}, nil
	}
	return o.disk.Open(name)
}

type unsavedFile struct {
	name   string
	reader *bytes.Reader
	size   int64
}

func (f *unsavedFile) Read(buffer []byte) (int, error) { return f.reader.Read(buffer) }
func (f *unsavedFile) Close() error                    { return nil }
func (f *unsavedFile) Stat() (fs.FileInfo, error) {
	return unsavedInfo{name: f.name, size: f.size}, nil
}

type unsavedInfo struct {
	name string
	size int64
}

func (i unsavedInfo) Name() string       { return i.name }
func (i unsavedInfo) Size() int64        { return i.size }
func (i unsavedInfo) Mode() fs.FileMode  { return 0o444 }
func (i unsavedInfo) ModTime() time.Time { return time.Time{} }
func (i unsavedInfo) IsDir() bool        { return false }
func (i unsavedInfo) Sys() any           { return nil }

func contentOf(files fs.FS, name string) []byte {
	opened, err := files.Open(name)
	if err != nil {
		return nil
	}
	defer opened.Close()
	content, err := io.ReadAll(opened)
	if err != nil {
		return nil
	}
	return content
}
