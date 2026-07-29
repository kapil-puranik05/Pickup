package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCommand = &cobra.Command{
	Use:   "client",
	Short: "CLI for interaction with the object storage",
}

func Execute() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCommand.AddCommand(uploadCommand)
	rootCommand.AddCommand(retrieveCommand)
	rootCommand.AddCommand(deleteCommand)
}
