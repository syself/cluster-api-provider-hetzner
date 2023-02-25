package exec

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// A dummy package to help with mocking out exec.NewCommand

type CmdRunner interface {
	Run(ctx context.Context, cmd string, args ...string) error
	RunIO(ctx context.Context, iostreams genericclioptions.IOStreams, cmd string, args ...string) error
}

type RealCmdRunner struct{}

func (RealCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	// For some reason, ExitError only gets populated with Stderr if we call Output().
	_, err := exec.CommandContext(ctx, cmd, args...).Output()

	return err
}

func (RealCmdRunner) RunIO(ctx context.Context, iostreams genericclioptions.IOStreams, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdin = iostreams.In
	c.Stderr = iostreams.ErrOut
	c.Stdout = iostreams.Out
	return c.Run()
}

type FakeCmdRunner struct {
	handler   func(argv []string) string
	LastArgs  []string
	LastStdin string
}

func NewFakeCmdRunner(handler func(argv []string) string) *FakeCmdRunner {
	return &FakeCmdRunner{handler: handler}
}

func (f *FakeCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	f.LastArgs = append([]string{cmd}, args...)
	_ = f.handler(append([]string{cmd}, args...))
	return nil
}

func (f *FakeCmdRunner) RunIO(ctx context.Context, iostreams genericclioptions.IOStreams, cmd string, args ...string) error {
	f.LastArgs = append([]string{cmd}, args...)

	if iostreams.In != nil {
		in, err := io.ReadAll(iostreams.In)
		if err != nil {
			return err
		}

		f.LastStdin = string(in)
	} else {
		f.LastStdin = ""
	}

	out := f.handler(append([]string{cmd}, args...))
	_, err := io.Copy(iostreams.Out, strings.NewReader(out))
	return err
}
