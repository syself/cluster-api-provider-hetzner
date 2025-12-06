/*
Copyright 2020 The Kubernetes Authors.

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

package version

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetReturnsVersionInfo(t *testing.T) {
	origMajor, origMinor := gitMajor, gitMinor
	origVersion, origCommit := gitVersion, gitCommit
	origTreeState, origBuildDate := gitTreeState, buildDate
	defer func() {
		gitMajor, gitMinor = origMajor, origMinor
		gitVersion, gitCommit = origVersion, origCommit
		gitTreeState, buildDate = origTreeState, origBuildDate
	}()

	gitMajor = "1"
	gitMinor = "2+"
	gitVersion = "v1.2.3"
	gitCommit = "abcdef"
	gitTreeState = "dirty"
	buildDate = "2025-01-02T15:04:05Z"

	info := Get()

	require.Equal(t, "1", info.Major)
	require.Equal(t, "2+", info.Minor)
	require.Equal(t, "v1.2.3", info.GitVersion)
	require.Equal(t, "abcdef", info.GitCommit)
	require.Equal(t, "dirty", info.GitTreeState)
	require.Equal(t, "2025-01-02T15:04:05Z", info.BuildDate)
	require.Equal(t, runtime.Version(), info.GoVersion)
	require.Equal(t, runtime.Compiler, info.Compiler)
	require.Equal(t, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), info.Platform)
}

func TestInfoString(t *testing.T) {
	info := Info{GitVersion: "v0.0.1"}
	require.Equal(t, "v0.0.1", info.String())
}
