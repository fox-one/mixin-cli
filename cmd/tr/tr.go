package tr

import (
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

var commands = []*cobra.Command{
	{
		Use:  "addressToAsset",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(addressToAssetID(args[0]))
		},
	},
	{
		Use:  "membersHash",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(mixinnet.HashMembers(args))
		},
	},
}

func Bind(root *cobra.Command) {
	for _, cmd := range commands {
		root.AddCommand(cmd)
	}
}
