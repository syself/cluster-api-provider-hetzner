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

// Package utils implements some utility functions.
//
//revive:disable:var-naming // Keep package name for compatibility with existing imports.
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var imageURLCommandNameRegex = regexp.MustCompile(`^image-url-command-[a-z0-9][a-z0-9._-]*$`)

// ValidateImageURLCommandName validates the user-provided command name. The name must be a
// basename, must not contain "..", and must start with image-url-command-.
func ValidateImageURLCommandName(name string) error {
	if name != filepath.Base(name) {
		return fmt.Errorf("must be a basename without slashes")
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("must not contain '..'")
	}

	if !imageURLCommandNameRegex.MatchString(name) {
		return fmt.Errorf("must match the regex %s", imageURLCommandNameRegex.String())
	}

	return nil
}

// ResolveImageURLCommandPath resolves a command name below the given directory.
func ResolveImageURLCommandPath(commandDir, name string) (string, error) {
	if err := ValidateImageURLCommandName(name); err != nil {
		return "", err
	}

	commandPath := filepath.Join(commandDir, name)
	if _, err := os.Stat(commandPath); err != nil {
		return "", fmt.Errorf("failed to stat %q: %w", commandPath, err)
	}

	return commandPath, nil
}
