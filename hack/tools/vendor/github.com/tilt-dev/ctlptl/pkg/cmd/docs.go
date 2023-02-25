package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newDocsCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "docs [path]",
		Short:  "Generate the markdown docs for ctlptl at [path]",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			err := doc.GenMarkdownTree(root, args[0])
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}
