package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/syself/cluster-api-provider-hetzner/test/e2e"
)

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	machineName := fs.String("machine-name", "manual-machine", "Machine name used in output paths")
	outputDir := fs.String("output-dir", "_artifacts/manual-machine-logs", "Directory for collected logs")
	timeout := fs.Duration("timeout", 10*time.Minute, "Timeout for log collection")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Collect logs from a CAPH machine over SSH using the e2e log collector.")
		fmt.Fprintf(os.Stderr, "Requires environment variable %s to contain the private key.\n", e2e.HetznerPrivateKeyContent)
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <host>\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "missing required argument: <host>")
		fs.Usage()
		os.Exit(2)
	}
	host := fs.Arg(0)

	if os.Getenv(e2e.HetznerPrivateKeyContent) == "" {
		fmt.Fprintf(os.Stderr, "missing required environment variable: %s\n", e2e.HetznerPrivateKeyContent)
		fs.Usage()
		os.Exit(2)
	}

	if err := os.MkdirAll(*outputDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := e2e.CollectMachineLogByExternalIP(ctx, *machineName, host, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "collect logs: %v\n", err)
		os.Exit(1)
	}

	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Printf("logs collected in %s\n", *outputDir)
		return
	}

	fmt.Printf("logs collected in %s\n", absOutputDir)
}
