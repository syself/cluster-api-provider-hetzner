/*
Copyright 2026 The Kubernetes Authors.

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

package imageurlcommand

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantMessage string
		wantErr     bool
	}{
		{
			name:        "message field is extracted",
			content:     `{"message":"downloading image"}`,
			wantMessage: "downloading image",
		},
		{
			name:        "unrelated fields are ignored",
			content:     `{"status":"Succeeded","message":"done","percentOfTimeout":42}`,
			wantMessage: "done",
		},
		{
			name:        "missing message field yields empty message",
			content:     `{"status":"Succeeded"}`,
			wantMessage: "",
		},
		{
			name:        "empty object yields empty message",
			content:     `{}`,
			wantMessage: "",
		},
		{
			name:    "empty string is invalid JSON",
			content: "",
			wantErr: true,
		},
		{
			name:    "malformed JSON returns an error",
			content: `{"message":`,
			wantErr: true,
		},
		{
			name:    "non-object JSON returns an error",
			content: `"just a string"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := Parse(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected an error, got none", tt.content)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) returned unexpected error: %v", tt.content, err)
			}
			if output.Message != tt.wantMessage {
				t.Errorf("Parse(%q).Message = %q, want %q", tt.content, output.Message, tt.wantMessage)
			}
		})
	}
}
