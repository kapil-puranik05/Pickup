package cmd

import (
	"client/internal"

	"github.com/spf13/cobra"
)

var uploadCommand = &cobra.Command{
	Use:   "upload [filename]",
	Short: "Upload Object",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := internal.UploadFile(args[0]); err != nil {
			return err
		}
		return nil
	},
}
