package transfer

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/pkg/qrcode"
	"github.com/manifoldco/promptui"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdTransfer() *cobra.Command {
	var opt struct {
		input  mixin.TransferInput
		amount string
		qrcode bool
		yes    bool
	}

	cmd := &cobra.Command{
		Use: "transfer",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			input := opt.input
			input.Amount, _ = decimal.NewFromString(opt.amount)

			if input.TraceID == "" {
				input.TraceID = mixin.RandomTraceID()
			}

			if opt.qrcode && input.OpponentID == "" {
				input.OpponentID = client.ClientID
			}

			if !input.Amount.IsPositive() {
				return errors.New("amount must be positive")
			}

			asset, err := client.ReadAsset(ctx, input.AssetID)
			if err != nil {
				return fmt.Errorf("read asset failed: %w", err)
			}

			if opt.qrcode {
				url := mixin.URL.Pay(&input)
				if len(input.OpponentMultisig.Receivers) > 0 {
					payment, err := client.VerifyPayment(ctx, input)
					if err != nil {
						return fmt.Errorf("verify payment failed: %w", err)
					}

					url = mixin.URL.Codes(payment.CodeID)
				}

				cmd.Println(url)
				qrcode.Print(url)
				return nil
			}

			pin, err := cmdutil.GetOrReadPin(s)
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}

			var (
				receiverNames []string
				execute       func() (interface{}, error)
			)

			if count := len(input.OpponentMultisig.Receivers); count > 0 {
				if t := int(input.OpponentMultisig.Threshold); t <= 0 || t > count {
					return errors.New("threshold must be in range [1, receivers count]")
				}

				for _, id := range input.OpponentMultisig.Receivers {
					user, err := client.ReadUser(ctx, id)
					if err != nil {
						return fmt.Errorf("read user failed: %w", err)
					}

					receiverNames = append(receiverNames, user.FullName)
					execute = func() (interface{}, error) {
						return client.Transaction(ctx, &input, pin)
					}
				}
			} else {
				user, err := client.ReadUser(ctx, input.OpponentID)
				if err != nil {
					return fmt.Errorf("read user failed: %w", err)
				}

				receiverNames = append(receiverNames, user.FullName)
				execute = func() (interface{}, error) {
					return client.Transfer(ctx, &input, pin)
				}
			}

			cmd.Printf("Transfer %s %s to %s\n", input.Amount, asset.Symbol, receiverNames)

			if confirmRequired := !(opt.yes || opt.qrcode); confirmRequired && !conformTransfer() {
				return nil
			}

			result, err := execute()
			if err != nil {
				return fmt.Errorf("transfer failed: %w", err)
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))

			return nil
		},
	}

	cmd.Flags().StringVar(&opt.input.AssetID, "asset", "", "asset id")
	cmd.Flags().StringVar(&opt.amount, "amount", "", "amount")
	cmd.Flags().StringVar(&opt.input.TraceID, "trace", "", "trace id")
	cmd.Flags().StringVar(&opt.input.Memo, "memo", "", "memo")
	cmd.Flags().StringVar(&opt.input.OpponentID, "opponent", "", "opponent id")
	cmd.Flags().StringSliceVar(&opt.input.OpponentMultisig.Receivers, "receivers", nil, "multisig receivers")
	cmd.Flags().Uint8Var(&opt.input.OpponentMultisig.Threshold, "threshold", 0, "multisig threshold")
	cmd.Flags().BoolVar(&opt.qrcode, "qrcode", false, "show qrcode")
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
