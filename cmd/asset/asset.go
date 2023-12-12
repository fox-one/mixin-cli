package asset

import (
	"github.com/fox-one/mixin-cli/v2/cmd/asset/list"
	"github.com/fox-one/mixin-cli/v2/cmd/asset/search"
	"github.com/spf13/cobra"
)

func NewCmdAsset() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "manager assets",
	}

	cmd.AddCommand(list.NewCmdList())
	cmd.AddCommand(search.NewCmdSearch())
	return cmd
}
