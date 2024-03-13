package safe

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/fox-one/pkg/uuid"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdTransfer() *cobra.Command {
	var opt struct {
		input  mixin.TransferInput
		amount string
		yes    bool
	}

	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "transfer",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			input := opt.input
			input.Amount, _ = decimal.NewFromString(opt.amount)

			if !input.Amount.IsPositive() {
				return errors.New("amount must be positive")
			}

			asset, err := client.SafeReadAsset(ctx, input.AssetID)
			if err != nil {
				return fmt.Errorf("read asset failed: %w", err)
			}

			var (
				tx  *mixinnet.Transaction
				raw string
			)
			if input.TraceID != "" {
				request, err := client.SafeReadMultisigRequests(ctx, input.TraceID)
				if err == nil {
					raw = request.RawTransaction
					tx, err = mixinnet.TransactionFromRaw(raw)
					if err != nil {
						return fmt.Errorf("parse transaction failed: %w", err)
					}
				} else if !mixin.IsErrorCodes(err, 404) {
					return fmt.Errorf("read multisig request failed: %w", err)
				}
			}

			if input.TraceID == "" {
				input.TraceID = mixin.RandomTraceID()
				cmd.Println("trace id:", input.TraceID)
			}

			if tx == nil {
				outputs, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
					State: mixin.SafeUtxoStateUnspent,
					Asset: asset.KernelAssetID,
					Limit: 256,
				})
				if err != nil {
					return fmt.Errorf("list unspent outputs failed: %w", err)
				}

				if len(outputs) > 256 {
					outputs = outputs[:256]
				}
				balance := decimal.Zero
				for i, utxo := range outputs {
					if balance = balance.Add(utxo.Amount); balance.GreaterThan(input.Amount) {
						outputs = outputs[:i+1]
						break
					}
				}
				if balance.LessThan(input.Amount) {
					if len(outputs) < 256 {
						return errors.New("insufficient balance")
					} else {
						cmd.Println("insufficient balance, try to merge 256 outputs?")
						// if !conformContinue() {
						// 	return errors.New("insufficient balance")
						// }
					}

					input.TraceID = mixin.RandomTraceID()
					cmd.Println("trace id:", input.TraceID)
					input.Amount = balance
					input.OpponentMultisig.Receivers = []string{client.ClientID}
					input.OpponentMultisig.Threshold = 1
					input.Memo = "merge outputs"
				}

				var (
					receiverNames []string
					receiver      *mixin.MixAddress
				)

				if count := len(input.OpponentMultisig.Receivers); count > 0 {
					if t := int(input.OpponentMultisig.Threshold); t <= 0 || t > count {
						return errors.New("threshold must be in range [1, receivers count]")
					}

					if _, err := uuid.FromString(input.OpponentMultisig.Receivers[0]); err != nil {
						receiver = mixin.RequireNewMainnetMixAddress(input.OpponentMultisig.Receivers, byte(input.OpponentMultisig.Threshold))
						receiverNames = input.OpponentMultisig.Receivers
					} else {
						receiver = mixin.RequireNewMixAddress(input.OpponentMultisig.Receivers, byte(input.OpponentMultisig.Threshold))
						for _, id := range input.OpponentMultisig.Receivers {
							user, err := client.ReadUser(ctx, id)
							if err != nil {
								return fmt.Errorf("read user failed: %w", err)
							}

							receiverNames = append(receiverNames, user.FullName)
						}
					}

				} else {
					user, err := client.ReadUser(ctx, input.OpponentID)
					if err != nil {
						return fmt.Errorf("read user failed: %w", err)
					}
					receiver = mixin.RequireNewMixAddress([]string{input.OpponentID}, 1)

					receiverNames = append(receiverNames, user.FullName)
				}

				cmd.Printf("Transfer %s %s to %s\n", input.Amount, asset.Symbol, receiverNames)

				builder := mixin.NewSafeTransactionBuilder(outputs)
				builder.Memo = input.Memo
				builder.Hint = input.TraceID
				tx, err = client.MakeTransaction(ctx, builder, []*mixin.TransactionOutput{
					{
						Address: receiver,
						Amount:  input.Amount,
					},
				})
				if err != nil {
					cmd.Println("MakeSafeTransaction error:", err)
					return fmt.Errorf("make safe transaction failed: %w", err)
				}

				raw, err = tx.Dump()
				if err != nil {
					return fmt.Errorf("dump transaction failed: %w", err)
				}
			}

			bts, _ := json.MarshalIndent(tx, "", "  ")
			cmd.Println(string(bts))

			cmd.Println("raw transaction:", raw)

			if confirmRequired := !(opt.yes); confirmRequired && !conformContinue() {
				return nil
			}

			spend, err := cmdutil.GetOrSpendKey(s)
			if err != nil {
				return fmt.Errorf("read spend key failed: %w", err)
			}

			request, err := client.SafeCreateTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
				RequestID:      input.TraceID,
				RawTransaction: raw,
			})
			if err != nil {
				return fmt.Errorf("create transaction request failed: %w", err)
			}

			if err := mixin.SafeSignTransaction(tx, *spend, request.Views, 0); err != nil {
				return fmt.Errorf("sign transaction failed: %w", err)
			}
			raw, err = tx.Dump()
			if err != nil {
				return fmt.Errorf("dump signed transaction failed: %w", err)
			}

			cmd.Println("signed transaction:", raw)

			request, err = client.SafeSubmitTransactionRequest(ctx, &mixin.SafeTransactionRequestInput{
				RequestID:      input.TraceID,
				RawTransaction: raw,
			})
			if err != nil {
				return fmt.Errorf("submit transaction request failed: %w", err)
			}

			cmd.Println("transaction hash:", request.TransactionHash)
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
	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve payment automatically")

	return cmd
}
