package create

import (
	"fmt"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/spf13/cobra"
)

func NewCmdCreate() *cobra.Command {
	var opt struct {
		pin string
	}

	cmd := &cobra.Command{
		Use:   "create <fullname>",
		Short: "create new user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			_, store, err := client.CreateUser(ctx, mixin.GenerateEd25519Key(), args[0])
			if err != nil {
				return err
			}

			newClient, err := mixin.NewFromKeystore(store)
			if err != nil {
				return err
			}

			pin := opt.pin
			if pin == "" {
				pin = mixin.RandomPin()
			}

			if err := newClient.ModifyPin(ctx, "", pin); err != nil {
				return err
			}

			storeWithPin := cmdutil.Keystore{
				Keystore: store,
				Pin:      pin,
			}

			fmt.Fprintln(cmd.OutOrStdout(), storeWithPin.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&opt.pin, "pin", "", "mixin wallet pin")
	return cmd
}
