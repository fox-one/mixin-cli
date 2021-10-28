package search

import (
	"encoding/json"
	"fmt"

	"github.com/fox-one/mixin-cli/session"
	"github.com/spf13/cobra"
)

func NewCmdSearch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <mixin id or identity number>",
		Short: "search user by mixin id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			user, err := client.ReadUser(ctx, args[0])
			if err != nil {
				return err
			}

			data, _ := json.MarshalIndent(user, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	return cmd
}
