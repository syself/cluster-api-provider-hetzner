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

// Package main contains main function for the e2e cli tool.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/syself/cluster-api-provider-hetzner/test/e2e"
)

func main() {
	err := do()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func do() error {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	machineName := fs.String("machine-name", "manual-machine", "Machine name used in output paths")
	outputDir := fs.String("output-dir", "_artifacts/manual-machine-logs", "Directory for collected logs")
	sshPrivKey := fs.String("ssh-private-key-file", "", fmt.Sprintf("SSH private key file path. If set, content gets base64-encoded and exported to %s", e2e.HetznerPrivateKeyContent))
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Collect logs from a CAPH machine over SSH using the e2e log collector.")
		fmt.Fprintf(os.Stderr, "Requires environment variable %s to contain base64-encoded private key content.\n", e2e.HetznerPrivateKeyContent)
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <host>\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly 1 host argument, got %d", fs.NArg())
	}
	host := fs.Arg(0)

	if sshPrivKey == nil || *sshPrivKey == "" {
		privKey := os.Getenv(e2e.HetznerPrivateKeyContent)
		if privKey == "" {
			fs.Usage()
			return fmt.Errorf("missing required environment variable: %s", e2e.HetznerPrivateKeyContent)
		}
		fmt.Printf("Using env var %s", e2e.HetznerPrivateKeyContent)
	} else {
		privKey, err := os.ReadFile(*sshPrivKey)
		if err != nil {
			return err
		}
		err = os.Setenv(e2e.HetznerPrivateKeyContent, base64.StdEncoding.EncodeToString(privKey))
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(*outputDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	if err := e2e.CollectMachineLogByExternalIP(context.Background(), *machineName, host, *outputDir); err != nil {
		fmt.Printf("logs collected in %s\n", *outputDir)

		return fmt.Errorf("collect logs: %w", err)
	}

	fmt.Printf("logs collected in %s\n", *outputDir)
	return nil
}
