package keystore

import (
	"encoding/hex"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/manifoldco/promptui"
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
			key := mixinnet.GenerateEd25519Key()
			seed := hex.EncodeToString(key.Seed())
			cmd.Println("ed25519_key:", hex.EncodeToString(key))
			cmd.Println("seed:", seed)

			{
				key, err := mixinnet.KeyFromSeed(seed)
				if err != nil {
					return err
				}
				cmd.Println("key:", key)
			}
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
	var opt struct {
		yes bool
	}

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

			cmd.Println("new pin:", newPin)

			{
				newPin := newPin
				if len(newPin) > 6 {
					key, err := mixinnet.KeyFromString(newPin)
					if err != nil {
						cmd.PrintErrf("invalid pin: %s", newPin)
						return err
					}
					newPin = key.Public().String()
				}

				if !opt.yes && !conformContinue() {
					return nil
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

	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve update pin automatically")
	return cmd
}

func conformContinue() bool {
	prompt := promptui.Prompt{
		Label:     "Continue",
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if err != nil {
		return false
	}

	switch result {
	case "y", "Y":
		return true
	default:
		return false
	}
}
