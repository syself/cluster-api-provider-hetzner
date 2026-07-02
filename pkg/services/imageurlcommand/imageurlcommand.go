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

// Package imageurlcommand contains shared logic for the image-url-command protocol.
package imageurlcommand

import (
	"encoding/json"
	"fmt"
)

// Output is the format of /root/output.json written by the image-url-command binary. Written
// continuously during execution; presence and content are optional from CAPH's perspective. On
// completion (success or failure) CAPH emits the full JSON as a Kubernetes Event (reason
// "ImageURLCommandOutputJSON") and logs it to the controller log. CAPH reads only the top level
// field "message" to write it to the corresponding message of the corresponding condition.
type Output struct {
	Message string `json:"message,omitempty"`
}

// Parse unmarshals content into an Output struct without applying conditions.
func Parse(content string) (Output, error) {
	var output Output
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return Output{}, fmt.Errorf("output.json: %w", err)
	}
	return output, nil
}
