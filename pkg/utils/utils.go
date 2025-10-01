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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// RemoveOwnerRefFromList removes the owner reference of a Kubernetes object.
func RemoveOwnerRefFromList(refList []metav1.OwnerReference, name, kind, apiVersion string) []metav1.OwnerReference {
	if len(refList) == 0 {
		return refList
	}
	index, found := FindOwnerRefFromList(refList, name, kind, apiVersion)
	// if owner ref is not found, return
	if !found {
		return refList
	}

	// if it is the only owner ref, we can return an empty slice
	if len(refList) == 1 {
		return []metav1.OwnerReference{}
	}

	// remove owner ref from slice
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList = refList[:refListLen]

	return RemoveOwnerRefFromList(refList, name, kind, apiVersion)
}

// FindOwnerRefFromList finds the owner ref of a Kubernetes object in a list of owner refs.
func FindOwnerRefFromList(refList []metav1.OwnerReference, name, kind, apiVersion string) (ref int, found bool) {
	bGV, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		panic("object has invalid group version")
	}

	for i, curOwnerRef := range refList {
		aGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			// ignore owner ref if it has invalid group version
			continue
		}

		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if curOwnerRef.Name == name &&
			curOwnerRef.Kind == kind &&
			aGV.Group == bGV.Group {
			return i, true
		}
	}
	return 0, false
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

// Compare two ResourceVersions, and return true if the local cache is up-to-date or ahead.
// Related: https://github.com/kubernetes-sigs/controller-runtime/issues/3320
func IsLocalCacheUpToDate(rvLocalCache, rvApiServer string) bool {
	if len(rvLocalCache) < len(rvApiServer) {
		// RV of cache is behind.
		return false
	}
	if len(rvLocalCache) > len(rvApiServer) {
		// RV of cache has changed like from "999" to "1000"
		return true
	}
	if rvLocalCache >= rvApiServer {
		// RV of cache is equal, or ahead.
		return true
	}
	return false
}
