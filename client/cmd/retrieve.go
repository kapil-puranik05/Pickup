package cmd

import (
	"client/internal"

	"github.com/spf13/cobra"
)

var retrieveCommand = &cobra.Command{
	Use:   "retrieve [filename]",
	Short: "Retreive Object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := internal.RetrieveFile(args[0]); err != nil {
			return err
		}
		return nil
	},
}
