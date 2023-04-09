package keystore

import (
	"github.com/spf13/cobra"
)

func NewCmdKeystore() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keystore",
		Short: "update keystore",
	}

	cmd.AddCommand(NewCmdPin())
	cmd.AddCommand(NewCmdUpdatePrivateKey())
	return cmd
}
