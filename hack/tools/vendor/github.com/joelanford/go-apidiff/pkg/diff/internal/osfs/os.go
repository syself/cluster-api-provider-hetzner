/*
Copyright 2021 Joe Lanford.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

NOTE: Originally copied from https://raw.githubusercontent.com/go-git/go-billy/d7a8afccaed297c30f8dff5724dbe422b491dd0d/osfs/os.go
*/

package osfs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-billy/v5"
)

const (
	defaultDirectoryMode = 0755
	defaultCreateMode    = 0666
)

// OS is a filesystem based on the os filesystem.
type OS struct {
	absWorkingDir string
}

// New returns a new OS filesystem.
func New(baseDir string) (billy.Filesystem, error) {
	fullPath, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	return &OS{absWorkingDir: fullPath}, nil
}

func (fs *OS) Chroot(path string) (billy.Filesystem, error) {
	path = fs.path(path)
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return New(fullPath)
}

func (fs *OS) Root() string {
	return fs.absWorkingDir
}

func (fs *OS) Create(filename string) (billy.File, error) {
	filename = fs.path(filename)
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *OS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	filename = fs.path(filename)
	if flag&os.O_CREATE != 0 {
		if err := fs.createDir(filename); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, err
}

func (fs *OS) createDir(fullpath string) error {
	fullpath = fs.path(fullpath)
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

func (fs *OS) ReadDir(path string) ([]os.FileInfo, error) {
	path = fs.path(path)
	l, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var s = make([]os.FileInfo, len(l))
	for i, f := range l {
		s[i] = f
	}

	return s, nil
}

func (fs *OS) Rename(from, to string) error {
	from, to = fs.path(from), fs.path(to)
	if err := fs.createDir(to); err != nil {
		return err
	}

	return rename(from, to)
}

func (fs *OS) MkdirAll(path string, perm os.FileMode) error {
	path = fs.path(path)
	return os.MkdirAll(path, defaultDirectoryMode)
}

func (fs *OS) Open(filename string) (billy.File, error) {
	filename = fs.path(filename)
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *OS) Stat(filename string) (os.FileInfo, error) {
	filename = fs.path(filename)
	return os.Stat(filename)
}

func (fs *OS) Remove(filename string) error {
	filename = fs.path(filename)
	return os.Remove(filename)
}

func (fs *OS) TempFile(dir, prefix string) (billy.File, error) {
	dir = fs.path(dir)
	if err := fs.createDir(dir + string(os.PathSeparator)); err != nil {
		return nil, err
	}

	f, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, nil
}

func (fs *OS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *OS) RemoveAll(path string) error {
	path = fs.path(path)
	return os.RemoveAll(path)
}

func (fs *OS) Lstat(filename string) (os.FileInfo, error) {
	filename = fs.path(filename)
	return os.Lstat(filename)
}

func (fs *OS) Symlink(target, link string) error {
	link = fs.path(link)
	if err := fs.createDir(link); err != nil {
		return err
	}

	return os.Symlink(target, link)
}

func (fs *OS) Readlink(link string) (string, error) {
	link = fs.path(link)
	return os.Readlink(link)
}

// Capabilities implements the Capable interface.
func (fs *OS) Capabilities() billy.Capability {
	return billy.DefaultCapabilities
}

func (fs *OS) path(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(fs.absWorkingDir, path)
	}
	return filepath.Clean(path)
}

// file is a wrapper for an os.File which adds support for file locking.
type file struct {
	*os.File
	m sync.Mutex
}
