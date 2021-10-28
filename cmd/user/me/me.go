package me

import (
	"encoding/json"
	"fmt"

	"github.com/fox-one/mixin-cli/session"
	"github.com/spf13/cobra"
)

func NewCmdMe() *cobra.Command {
	cmd := &cobra.Command{
		Use: "me",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			me, err := client.UserMe(ctx)
			if err != nil {
				return err
			}

			data, _ := json.MarshalIndent(me, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	return cmd
}
