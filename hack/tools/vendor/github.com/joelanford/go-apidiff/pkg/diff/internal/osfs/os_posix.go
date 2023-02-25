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

NOTE: Originally copied from https://raw.githubusercontent.com/go-git/go-billy/d7a8afccaed297c30f8dff5724dbe422b491dd0d/osfs/os_posix.go
*/

// +build !plan9,!windows

package osfs

import (
	"os"

	"golang.org/x/sys/unix"
)

func (f *file) Lock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.Flock(int(f.File.Fd()), unix.LOCK_EX)
}

func (f *file) Unlock() error {
	f.m.Lock()
	defer f.m.Unlock()

	return unix.Flock(int(f.File.Fd()), unix.LOCK_UN)
}

func rename(from, to string) error {
	return os.Rename(from, to)
}
