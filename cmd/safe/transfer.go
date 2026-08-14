package safe

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdTransfer() *cobra.Command {
	var opt safeTransferOptions

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
			if err := cmdutil.NormalizeMultisigDestination(&opt.input); err != nil {
				return err
			}

			if shouldContinueSafeMultisigTransfer(cmd, opt) {
				request, err := readSafeMultisigRequest(ctx, client, opt.input.TraceID)
				if err == nil {
					return continueSafeMultisigTransfer(cmd, client, request, opt)
				}
				if !mixin.IsErrorCodes(err, 404) {
					return fmt.Errorf("read multisig request failed: %w", err)
				}
			}

			input := opt.input
			input.Amount, _ = decimal.NewFromString(opt.amount)

			if !input.Amount.IsPositive() {
				return errors.New("amount must be positive")
			}
			if input.AssetID == "" {
				return errors.New("asset is required")
			}
			if opt.isMultisigSource() {
				return createSafeMultisigTransfer(cmd, client, input, opt)
			}

			asset, err := client.SafeReadAsset(ctx, input.AssetID)
			if err != nil {
				return fmt.Errorf("read asset failed: %w", err)
			}
			kernelAsset, err := mixinnet.HashFromString(asset.KernelAssetID)
			if err != nil {
				return fmt.Errorf("invalid kernel asset id: %w", err)
			}

			var (
				tx  *mixinnet.Transaction
				raw string
			)

			if input.TraceID == "" {
				input.TraceID = mixin.RandomTraceID()
				cmd.Println("trace id:", input.TraceID)
			}

			if tx == nil {
				outputs, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
					State: mixin.SafeUtxoStateUnspent,
					Asset: asset.KernelAssetID,
					Limit: safeTransactionInputLimit,
				})
				if err != nil {
					return fmt.Errorf("list unspent outputs failed: %w", err)
				}

				if len(outputs) > safeTransactionInputLimit {
					outputs = outputs[:safeTransactionInputLimit]
				}
				balance := decimal.Zero
				for i, utxo := range outputs {
					if utxo == nil {
						return errors.New("invalid safe output response: nil output")
					}
					if utxo.State != "" && utxo.State != mixin.SafeUtxoStateUnspent {
						return fmt.Errorf("invalid safe output %s: state %s", utxo.OutputID, utxo.State)
					}
					if !utxo.Amount.IsPositive() {
						return fmt.Errorf("invalid safe output %s: non-positive amount", utxo.OutputID)
					}
					if balance = balance.Add(utxo.Amount); !balance.LessThan(input.Amount) {
						outputs = outputs[:i+1]
						break
					}
				}
				if balance.LessThan(input.Amount) {
					if len(outputs) < safeTransactionInputLimit {
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
				if err := validateSafeSourceOutputs(outputs, []string{client.ClientID}, 1, kernelAsset); err != nil {
					return err
				}

				receiver, receiverNames, err := safeTransferReceiver(ctx, client, input)
				if err != nil {
					return err
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

			requests, err := client.SafeCreateTransactionRequests(ctx, []*mixin.SafeTransactionRequestInput{{
				RequestID:      input.TraceID,
				RawTransaction: raw,
			}})
			if err != nil {
				return fmt.Errorf("create transaction request failed: %w", err)
			}
			if len(requests) != 1 || requests[0] == nil {
				return fmt.Errorf("create transaction request returned %d valid requests, want 1", len(requests))
			}
			request := requests[0]
			if err := validateSafeTransactionResponse(request, input, []string{client.ClientID}, 1, tx, raw, false); err != nil {
				return fmt.Errorf("validate created transaction request failed: %w", err)
			}
			if len(request.Views) != len(tx.Inputs) {
				return fmt.Errorf("invalid transaction views: got %d for %d inputs", len(request.Views), len(tx.Inputs))
			}
			for i, view := range request.Views {
				if _, err := view.ToScalar(); err != nil {
					return fmt.Errorf("invalid transaction view %d: %w", i, err)
				}
			}

			if err := mixin.SafeSignTransaction(tx, *spend, request.Views, 0); err != nil {
				return fmt.Errorf("sign transaction failed: %w", err)
			}
			raw, err = tx.Dump()
			if err != nil {
				return fmt.Errorf("dump signed transaction failed: %w", err)
			}

			cmd.Println("signed transaction:", raw)

			requests, err = client.SafeSubmitTransactionRequests(ctx, []*mixin.SafeTransactionRequestInput{{
				RequestID:      input.TraceID,
				RawTransaction: raw,
			}})
			if err != nil {
				return fmt.Errorf("submit transaction request failed: %w", err)
			}
			if len(requests) != 1 || requests[0] == nil {
				return fmt.Errorf("submit transaction request returned %d valid requests, want 1", len(requests))
			}
			request = requests[0]
			if err := validateSafeTransactionResponse(request, input, []string{client.ClientID}, 1, tx, raw, true); err != nil {
				return fmt.Errorf("validate submitted transaction request failed: %w", err)
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
	cmd.Flags().StringSliceVar(&opt.input.OpponentMultisig.Receivers, "receivers", nil, "multisig receiver members or one MIX address")
	cmd.Flags().Uint8Var(&opt.input.OpponentMultisig.Threshold, "threshold", 0, "multisig threshold")
	cmd.Flags().StringSliceVar(&opt.senders, "senders", nil, "source multisig members")
	cmd.Flags().Uint8Var(&opt.senderThreshold, "sender-threshold", 0, "source multisig threshold")
	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve payment automatically")

	cmd.AddCommand(newCmdCancelSafeMultisigSignature())

	return cmd
}

type safeTransferOptions struct {
	input           mixin.TransferInput
	amount          string
	senders         []string
	senderThreshold uint8
	yes             bool
}

func (opt safeTransferOptions) isMultisigSource() bool {
	return len(opt.senders) > 0 || opt.senderThreshold > 0
}

func shouldContinueSafeMultisigTransfer(cmd *cobra.Command, opt safeTransferOptions) bool {
	if opt.input.TraceID == "" {
		return false
	}
	if opt.isMultisigSource() {
		return true
	}
	for _, name := range []string{"asset", "amount", "memo", "opponent", "receivers", "threshold"} {
		if cmd.Flags().Changed(name) {
			return false
		}
	}
	return true
}

func validateSafeTransactionResponse(request *mixin.SafeTransactionRequest, input mixin.TransferInput, senders []string, senderThreshold uint8, tx *mixinnet.Transaction, raw string, requireHash bool) error {
	if request == nil {
		return errors.New("empty safe transaction response")
	}
	if tx == nil || len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return errors.New("invalid local safe transaction")
	}
	if request.RequestID != input.TraceID {
		return fmt.Errorf("request id mismatch: expected %s, got %s", input.TraceID, request.RequestID)
	}
	if request.RawTransaction == "" {
		return errors.New("safe transaction response has no raw transaction")
	}
	if err := validateSafeRequestRaw(request.RawTransaction, raw); err != nil {
		return err
	}
	responseTx, err := mixinnet.TransactionFromRaw(request.RawTransaction)
	if err != nil {
		return fmt.Errorf("parse safe transaction response failed: %w", err)
	}
	if err := validateSafeTransactionSigners(responseTx, request.Signers, senders); err != nil {
		return err
	}
	if err := validateSafeSignaturePreservation(raw, request.RawTransaction); err != nil {
		return err
	}

	responseAsset := request.KernelAssetID
	if !responseAsset.HasValue() {
		responseAsset = request.AssetID
	}
	if !responseAsset.HasValue() {
		responseAsset = request.Asset
	}
	if !responseAsset.HasValue() || responseAsset != tx.Asset {
		return errors.New("safe transaction response asset mismatch")
	}
	if !request.Amount.Equal(input.Amount) {
		return fmt.Errorf("amount mismatch: expected %s, got %s", input.Amount, request.Amount)
	}
	if request.Extra != input.Memo {
		return fmt.Errorf("memo mismatch: expected %q, got %q", input.Memo, request.Extra)
	}
	if request.SendersThreshold != senderThreshold || !sameSafeMembers(request.Senders, senders) {
		return errors.New("safe transaction response source mismatch")
	}
	if err := validateSafeTransactionOutputs(tx, request.Receivers, senders, senderThreshold); err != nil {
		return err
	}
	if err := validateSafeDestination(input); err != nil {
		return err
	}
	destinationMembers := input.OpponentMultisig.Receivers
	destinationThreshold := input.OpponentMultisig.Threshold
	if len(destinationMembers) == 0 {
		destinationMembers = []string{input.OpponentID}
		destinationThreshold = 1
	}
	if request.Receivers[0].Threshold != destinationThreshold || !sameSafeMembers(request.Receivers[0].Members, destinationMembers) {
		return errors.New("safe transaction response destination mismatch")
	}

	hash, err := tx.TransactionHash()
	if err != nil {
		return fmt.Errorf("hash local safe transaction failed: %w", err)
	}
	if requireHash && request.TransactionHash == "" {
		return errors.New("safe transaction response has no transaction hash")
	}
	if request.TransactionHash != "" && request.TransactionHash != hash.String() {
		return fmt.Errorf("transaction hash mismatch: expected %s, got %s", hash, request.TransactionHash)
	}
	return nil
}
