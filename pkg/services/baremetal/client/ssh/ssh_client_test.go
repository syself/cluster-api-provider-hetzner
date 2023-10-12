/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sshclient contains the interface to speak to bare metal servers with ssh.
package sshclient

import "testing"

func Test_removeUselessLinesFromCloudInitOutput(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "ignore: 10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s",
			s:    "foo\n 10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: ^10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s",
			s:    "foo\n10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: Get:17 http://archive.ubuntu.com/ubuntu focal/universe Translation-en [5,124 kB[]",
			s:    "foo\nGet:17 http://archive.ubuntu.com/ubuntu focal/universe Translation-en [5,124 kB[]\nbar",
			want: "foo\nbar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeUselessLinesFromCloudInitOutput(tt.s); got != tt.want {
				t.Errorf("removeUselessLinesFromCloudInitOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
