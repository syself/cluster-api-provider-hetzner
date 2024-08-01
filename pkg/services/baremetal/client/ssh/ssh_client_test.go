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

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_removeUselessLinesFromCloudInitOutput(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "ignore: 10000K ...",
			s:    "foo\n 10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: ^10000K ...2",
			s:    "foo\n10000K .......... .......... .......... .......... ..........  6%!M(MISSING) 1s\nbar",
			want: "foo\nbar",
		},
		{
			name: "ignore: Get:17 http://...",
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

func TestOutput_String(t *testing.T) {
	f := func(o Output, wantString string) {
		require.Equal(t, wantString, o.String())
	}
	f(Output{
		StdOut: "",
		StdErr: "",
		Err:    nil,
	}, "")

	f(Output{
		StdOut: " mystdout ",
		StdErr: "",
		Err:    nil,
	}, "mystdout")

	f(Output{
		StdOut: "",
		StdErr: " mystderr ",
		Err:    nil,
	}, "mystderr")

	f(Output{
		StdOut: "",
		StdErr: "",
		Err:    fmt.Errorf(" some err "),
	}, "some err")

	f(Output{
		StdOut: "mystdout",
		StdErr: "",
		Err:    fmt.Errorf("some err"),
	}, "mystdout. Err: some err")

	f(Output{
		StdOut: "",
		StdErr: "mystderr",
		Err:    fmt.Errorf("some err"),
	}, "mystderr. Err: some err")

	f(Output{
		StdOut: "mystdout",
		StdErr: "mystderr",
		Err:    fmt.Errorf("some err"),
	}, "mystdout. Stderr: mystderr. Err: some err")
}
