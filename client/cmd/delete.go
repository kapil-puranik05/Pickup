package cmd

import (
	"client/internal"

	"github.com/spf13/cobra"
)

var deleteCommand = &cobra.Command{
	Use:   "delete [filename]",
	Short: "Delete Object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := internal.DeleteFile(args[0]); err != nil {
			return err
		}
		return nil
	},
}
