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

NOTE: Originally copied from https://raw.githubusercontent.com/go-git/go-billy/d7a8afccaed297c30f8dff5724dbe422b491dd0d/osfs/os_windows.go
*/

// +build windows

package osfs

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileInfo struct {
	os.FileInfo
	name string
}

func (fi *fileInfo) Name() string {
	return fi.name
}

var (
	kernel32DLL    = windows.NewLazySystemDLL("kernel32.dll")
	lockFileExProc = kernel32DLL.NewProc("LockFileEx")
	unlockFileProc = kernel32DLL.NewProc("UnlockFile")
)

const (
	lockfileExclusiveLock = 0x2
)

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	var overlapped windows.Overlapped
	// err is always non-nil as per sys/windows semantics.
	ret, _, err := lockFileExProc.Call(f.File.Fd(), lockfileExclusiveLock, 0, 0xFFFFFFFF, 0,
		uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if ret == 0 {
		return err
	}
	return nil
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	// err is always non-nil as per sys/windows semantics.
	ret, _, err := unlockFileProc.Call(f.File.Fd(), 0, 0, 0xFFFFFFFF, 0)
	if ret == 0 {
		return err
	}
	return nil
}

func rename(from, to string) error {
	return os.Rename(from, to)
}
