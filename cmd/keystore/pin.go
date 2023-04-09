package keystore

import (
	"crypto/rand"

	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go"
	"github.com/spf13/cobra"
)

func NewCmdPin() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pin",
		Short: "keystore pin",
	}

	cmd.AddCommand(NewCmdCreateTipPin())
	cmd.AddCommand(NewCmdUpdatePin())
	cmd.AddCommand(NewCmdVerifyPin())
	return cmd
}

func NewCmdCreateTipPin() *cobra.Command {
	cmd := &cobra.Command{
		Use: "new-key",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("new tip pin:", mixin.NewKey(rand.Reader))
			return nil
		},
	}

	return cmd
}

func NewCmdVerifyPin() *cobra.Command {
	cmd := &cobra.Command{
		Use: "verify",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			if err := client.VerifyPin(ctx, s.GetPin()); err != nil {
				cmd.PrintErrf("verify pin failed: %s\n%s", s.GetPin(), err.Error())
				return err
			}

			cmd.Println("pin verified!")
			return nil
		},
	}

	return cmd
}

func NewCmdUpdatePin() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "update <new-pin>",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			newPin := args[0]
			if err := mixin.ValidatePinPattern(newPin); err != nil {
				cmd.PrintErrf("invalid pin: %s", newPin)
				return err
			}
			{
				newPin := newPin
				if len(newPin) > 6 {
					key, err := mixin.KeyFromString(newPin)
					if err != nil {
						cmd.PrintErrf("invalid pin: %s", newPin)
						return err
					}
					newPin = key.Public().String()
				}

				if err := client.ModifyPin(ctx, s.GetPin(), newPin); err != nil {
					cmd.PrintErrf("modify pin failed: %s", err.Error())
					return err
				}
			}

			cmd.Printf("pin updated: %s", newPin)
			return nil
		},
	}

	return cmd
}
