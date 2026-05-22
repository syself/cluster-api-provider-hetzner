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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateOfImageURLCommandV2Logic(t *testing.T) {
	someErr := errors.New("ssh error")

	tests := []struct {
		name               string
		pidExitCode        int
		pidErr             error
		psExitCode         int
		psErr              error
		outputJSONContent  string
		outputJSONErr      error
		wantState          ImageURLCommandState
		wantDetailContains string
		wantErr            bool
	}{
		{
			name:        "pid file missing → not started",
			pidExitCode: 1,
			wantState:   ImageURLCommandStateNotStarted,
		},
		{
			name:      "pid file check errors → not started with error",
			pidErr:    someErr,
			wantState: ImageURLCommandStateNotStarted,
			wantErr:   true,
		},
		{
			name:       "process still running → running",
			psExitCode: 0,
			wantState:  ImageURLCommandStateRunning,
		},
		{
			name:      "process check errors → not started with error",
			psErr:     someErr,
			wantState: ImageURLCommandStateNotStarted,
			wantErr:   true,
		},
		{
			name:               "process exited, output.json missing → failed",
			psExitCode:         1,
			outputJSONErr:      errors.New("exit status 1"),
			wantState:          ImageURLCommandStateFailed,
			wantDetailContains: "process exited",
		},
		{
			name:               "process exited, output.json not valid JSON → failed",
			psExitCode:         1,
			outputJSONContent:  "not-json",
			wantState:          ImageURLCommandStateFailed,
			wantDetailContains: "not-json",
		},
		{
			name:              "process exited, output.json has no Status → failed",
			psExitCode:        1,
			outputJSONContent: `{"apiVersion":"v2"}`,
			wantState:         ImageURLCommandStateFailed,
		},
		{
			name:              "process exited, output.json has Status → finished successfully",
			psExitCode:        1,
			outputJSONContent: `{"apiVersion":"v2","status":"Succeeded","phases":{}}`,
			wantState:         ImageURLCommandStateFinishedSuccessfully,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, detail, err := stateOfImageURLCommandV2Logic(
				tc.pidExitCode, tc.pidErr,
				tc.psExitCode, tc.psErr,
				tc.outputJSONContent, tc.outputJSONErr,
			)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantState, state)
			if tc.wantDetailContains != "" {
				require.Contains(t, detail, tc.wantDetailContains)
			}
		})
	}
}
