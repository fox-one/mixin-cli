package user

import (
	"github.com/fox-one/mixin-cli/cmd/user/create"
	"github.com/fox-one/mixin-cli/cmd/user/me"
	"github.com/fox-one/mixin-cli/cmd/user/search"
	"github.com/spf13/cobra"
)

func NewCmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "manager users",
	}

	cmd.AddCommand(create.NewCmdCreate())
	cmd.AddCommand(search.NewCmdSearch())
	cmd.AddCommand(me.NewCmdMe())

	return cmd
}
