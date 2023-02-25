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

NOTE: Originally copied from https://raw.githubusercontent.com/go-git/go-billy/d7a8afccaed297c30f8dff5724dbe422b491dd0d/osfs/os_plan9.go
*/

package osfs

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func (f *file) Lock() error {
	// Plan 9 uses a mode bit instead of explicit lock/unlock syscalls.
	//
	// Per http://man.cat-v.org/plan_9/5/stat: “Exclusive use files may be open
	// for I/O by only one fid at a time across all clients of the server. If a
	// second open is attempted, it draws an error.”
	//
	// There is no obvious way to implement this function using the exclusive use bit.
	// See https://golang.org/src/cmd/go/internal/lockedfile/lockedfile_plan9.go
	// for how file locking is done by the go tool on Plan 9.
	return nil
}

func (f *file) Unlock() error {
	return nil
}

func rename(from, to string) error {
	// If from and to are in different directories, copy the file
	// since Plan 9 does not support cross-directory rename.
	if filepath.Dir(from) != filepath.Dir(to) {
		fi, err := os.Stat(from)
		if err != nil {
			return &os.LinkError{"rename", from, to, err}
		}
		if fi.Mode().IsDir() {
			return &os.LinkError{"rename", from, to, syscall.EISDIR}
		}
		fromFile, err := os.Open(from)
		if err != nil {
			return &os.LinkError{"rename", from, to, err}
		}
		toFile, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
		if err != nil {
			return &os.LinkError{"rename", from, to, err}
		}
		_, err = io.Copy(toFile, fromFile)
		if err != nil {
			return &os.LinkError{"rename", from, to, err}
		}

		// Copy mtime and mode from original file.
		// We need only one syscall if we avoid os.Chmod and os.Chtimes.
		dir := fi.Sys().(*syscall.Dir)
		var d syscall.Dir
		d.Null()
		d.Mtime = dir.Mtime
		d.Mode = dir.Mode
		if err = dirwstat(to, &d); err != nil {
			return &os.LinkError{"rename", from, to, err}
		}

		// Remove original file.
		err = os.Remove(from)
		if err != nil {
			return &os.LinkError{"rename", from, to, err}
		}
		return nil
	}
	return os.Rename(from, to)
}

func dirwstat(name string, d *syscall.Dir) error {
	var buf [syscall.STATFIXLEN]byte

	n, err := d.Marshal(buf[:])
	if err != nil {
		return &os.PathError{"dirwstat", name, err}
	}
	if err = syscall.Wstat(name, buf[:n]); err != nil {
		return &os.PathError{"dirwstat", name, err}
	}
	return nil
}
