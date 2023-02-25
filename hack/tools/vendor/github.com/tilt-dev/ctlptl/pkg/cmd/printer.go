package cmd

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"

	myprinters "github.com/tilt-dev/ctlptl/internal/printers"
)

func toPrinter(flags *genericclioptions.PrintFlags) (printers.ResourcePrinter, error) {
	p, err := flags.ToPrinter()
	if err != nil {
		return nil, err
	}
	namePrinter, ok := p.(*printers.NamePrinter)
	if ok {
		return &myprinters.NamePrinter{
			ShortOutput: namePrinter.ShortOutput,
			Operation:   namePrinter.Operation,
		}, nil
	}
	return p, nil
}
