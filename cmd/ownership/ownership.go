package ownership

import (
	"github.com/spf13/cobra"
)

func NewCmdOwnership() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ownership",
		Short: "manage Dapp ownership, only available for tip PIN Dapp",
	}

	cmd.AddCommand(NewCmdTransfer())
	return cmd
}
