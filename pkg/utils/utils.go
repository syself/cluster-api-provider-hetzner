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

// Package utils implements some utility functions
package utils

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
			EncodeLevel:    zapcore.LowercaseColorLevelEncoder,
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
