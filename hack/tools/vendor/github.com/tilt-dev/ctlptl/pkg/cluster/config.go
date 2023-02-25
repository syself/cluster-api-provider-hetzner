package cluster

import (
	"os/exec"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type configWriter interface {
	SetContext(name string) error
	DeleteContext(name string) error
	SetConfig(name, value string) error
}

type kubeconfigWriter struct {
	iostreams genericclioptions.IOStreams
}

func (w kubeconfigWriter) SetContext(name string) error {
	cmd := exec.Command("kubectl", "config", "use-context", name)
	cmd.Stdout = w.iostreams.Out
	cmd.Stderr = w.iostreams.ErrOut
	return cmd.Run()
}

func (w kubeconfigWriter) DeleteContext(name string) error {
	cmd := exec.Command("kubectl", "config", "delete-context", name)
	cmd.Stdout = w.iostreams.Out
	cmd.Stderr = w.iostreams.ErrOut
	return cmd.Run()
}

func (w kubeconfigWriter) SetConfig(name, value string) error {
	cmd := exec.Command("kubectl", "config", "set", name, value)
	cmd.Stdout = w.iostreams.Out
	cmd.Stderr = w.iostreams.ErrOut
	return cmd.Run()
}
