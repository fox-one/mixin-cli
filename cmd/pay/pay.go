package pay

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/cmdutil"
	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go"
	"github.com/manifoldco/promptui"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdPay() *cobra.Command {
	var opt struct {
		memo string
		yes  bool
	}

	cmd := &cobra.Command{
		Use:   "pay",
		Short: "transfer assets to other users",
		Long:  "transfer {opponent_id} {asset_id} {amount} {memo}",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			opponentID := args[0]
			opponent, err := client.ReadUser(ctx, opponentID)
			if err != nil {
				return fmt.Errorf("read opponent with id %q failed: %w", opponentID, err)
			}

			assetID := args[1]
			asset, err := client.ReadAsset(ctx, assetID)
			if err != nil {
				return fmt.Errorf("read asset failed: %w", err)
			}

			amount, _ := decimal.NewFromString(args[2])
			if !amount.IsPositive() {
				return errors.New("amount must be positive")
			}

			memo := opt.memo
			if memo == "" && len(args) >= 4 {
				memo = args[3]
			}

			pin, err := cmdutil.GetOrReadPin(s)
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}

			cmd.Printf("transfer %s %s (balance %s) to %s with memo %q\n", amount, asset.Symbol, asset.Balance, opponent.FullName, memo)

			if opt.yes || conformTransfer() {
				snapshot, err := client.Transfer(ctx, &mixin.TransferInput{
					AssetID:    assetID,
					OpponentID: opponent.UserID,
					Amount:     amount,
					TraceID:    mixin.RandomTraceID(),
					Memo:       memo,
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

	cmd.Flags().StringVar(&opt.memo, "memo", "", "payment memo")
	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve payment automatically")

	return cmd
}

func conformTransfer() bool {
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
