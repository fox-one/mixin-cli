package withdraw

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/cmdutil"
	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/manifoldco/promptui"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdWithdraw() *cobra.Command {
	var opt struct {
		yes  bool
		tag  string
		memo string
	}

	cmd := &cobra.Command{
		Use:   "withdraw <asset> <amount> <address>",
		Short: "withdraw to another wallet address",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			assetID := args[0]
			asset, err := client.ReadAsset(ctx, assetID)
			if err != nil {
				return fmt.Errorf("read asset %s failed: %w", assetID, err)
			}

			chain := asset
			if asset.AssetID != asset.ChainID {
				chain, err = client.ReadAsset(ctx, asset.ChainID)
				if err != nil {
					return fmt.Errorf("read asset %s failed: %w", asset.ChainID, err)
				}
			}

			amount, _ := decimal.NewFromString(args[1])
			if !amount.IsPositive() {
				return errors.New("amount must be positive")
			}

			pin, err := cmdutil.GetOrReadPin(s)
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}

			address, err := client.CreateAddress(ctx, mixin.CreateAddressInput{
				AssetID:     asset.AssetID,
				Destination: args[2],
				Tag:         opt.tag,
				Label:       "by mixin-cli",
			}, pin)
			if err != nil {
				return fmt.Errorf("create address failed: %w", err)
			}

			cmd.Printf("withdraw %s %s (balance %s) to address %q & tag %q with fee %s %s (balance %s)\n",
				amount, asset.Symbol, asset.Balance, address.Destination, address.Tag, address.Fee, chain.Symbol, chain.Balance)

			if opt.yes || conformWithdraw() {
				snapshot, err := client.Withdraw(ctx, mixin.WithdrawInput{
					AddressID: address.AddressID,
					Amount:    amount,
					TraceID:   mixin.RandomTraceID(),
					Memo:      opt.memo,
				}, pin)

				if err != nil {
					return err
				}

				data, _ := json.MarshalIndent(snapshot, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opt.tag, "tag", "", "address tag")
	cmd.Flags().StringVar(&opt.memo, "memo", "", "payment memo")
	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve payment automatically")

	return cmd
}

func conformWithdraw() bool {
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
