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
package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apiserver/pkg/storage/names"
)

// LabelsToLabelSelector is converting a map of labels to HCloud label
// selector.
func LabelsToLabelSelector(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for key, val := range labels {
		parts = append(
			parts,
			fmt.Sprintf("%s==%s", key, val),
		)
	}
	return strings.Join(parts, ",")
}

// LabelSelectorToLabels is converting an HCloud label
// selector to a map of labels.
func LabelSelectorToLabels(str string) (map[string]string, error) {
	labels := make(map[string]string)
	if str == "" {
		return labels, nil
	}
	input := strings.ReplaceAll(str, "==", `":"`)
	input = strings.ReplaceAll(input, ",", `","`)
	input = fmt.Sprintf(`{"%s"}`, input) //nolint:gocritic

	if err := json.Unmarshal([]byte(input), &labels); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	return labels, nil
}

// DifferenceOfStringSlices returns the elements in `a` that aren't in `b` as well as elements of `a` not in `b`.
func DifferenceOfStringSlices(a, b []string) (onlyInA []string, onlyInB []string) {
	ma := make(map[string]struct{}, len(a))
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	for _, x := range a {
		ma[x] = struct{}{}
	}

	for _, x := range a {
		if _, found := mb[x]; !found {
			onlyInA = append(onlyInA, x)
		}
	}

	for _, x := range b {
		if _, found := ma[x]; !found {
			onlyInB = append(onlyInB, x)
		}
	}
	return
}

// DifferenceOfIntSlices returns the elements in `a` that aren't in `b` as well as elements of `a` not in `b`.
func DifferenceOfIntSlices(a, b []int) (onlyInA []int, onlyInB []int) {
	ma := make(map[int]struct{}, len(a))
	mb := make(map[int]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	for _, x := range a {
		ma[x] = struct{}{}
	}

	for _, x := range a {
		if _, found := mb[x]; !found {
			onlyInA = append(onlyInA, x)
		}
	}

	for _, x := range b {
		if _, found := ma[x]; !found {
			onlyInB = append(onlyInB, x)
		}
	}
	return
}

// StringInList returns a boolean indicating whether strToSearch is a
// member of the string slice passed as the first argument.
func StringInList(list []string, strToSearch string) bool {
	for _, item := range list {
		if item == strToSearch {
			return true
		}
	}
	return false
}

// FilterStringFromList produces a new string slice that does not
// include the strToFilter argument.
func FilterStringFromList(list []string, strToFilter string) (newList []string) {
	for _, item := range list {
		if item != strToFilter {
			newList = append(newList, item)
		}
	}
	return
}

// GenerateName takes a name as string pointer. It returns name if pointer is not nil, otherwise it returns fallback with random suffix.
func GenerateName(name *string, fallback string) string {
	if name != nil {
		return *name
	}
	return names.SimpleNameGenerator.GenerateName(fallback)
}

// GetDefaultLogger returns a default zapr logger.
func GetDefaultLogger(logLevel string) logr.Logger {
	cfg := zap.Config{
		Encoding:    "json",
		OutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:    "message",
			CallerKey:     "file",
			LevelKey:      "level",
			TimeKey:       "time",
			NameKey:       "logger",
			StacktraceKey: "stacktrace",

			LineEnding:     zapcore.DefaultLineEnding,
			EncodeCaller:   zapcore.ShortCallerEncoder,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeName:     zapcore.FullNameEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
		},
	}

	switch logLevel {
	case "error":
		cfg.Development = false
		cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "debug":
		cfg.Development = true
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		cfg.Development = true
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	zapLog, err := cfg.Build()
	if err != nil {
		log.Fatalf("Error while initializing zapLogger: %v", err)
	}

	return zapr.NewLogger(zapLog)
}
