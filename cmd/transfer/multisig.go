package transfer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

const legacyTransactionInputLimit = 256

type legacyOutputLister interface {
	ListMultisigOutputs(context.Context, mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error)
}

type legacyTransactionMaker interface {
	MakeTransaction(context.Context, *mixin.TransactionBuilder, []*mixin.TransactionOutput) (*mixinnet.Transaction, error)
}

type legacyMultisigClient interface {
	legacyOutputLister
	legacyTransactionMaker
	CreateMultisig(context.Context, string, string) (*mixin.MultisigRequest, error)
	SignMultisig(context.Context, string, string) (*mixin.MultisigRequest, error)
	UnlockMultisig(context.Context, string, string) error
	CancelMultisig(context.Context, string) error
	ReadAsset(context.Context, string) (*mixin.Asset, error)
	ReadUser(context.Context, string) (*mixin.User, error)
}

func runMultisigTransfer(cmd *cobra.Command, client *mixin.Client, input mixin.TransferInput, senders []string, senderThreshold uint8, yes bool) error {
	ctx := cmd.Context()
	if err := validateLegacyMultisigTransfer(input, senders, senderThreshold); err != nil {
		return err
	}
	if !containsString(senders, client.ClientID) {
		return errors.New("current user is not a source multisig member")
	}

	receiver, receiverNames, err := legacyTransferReceiver(ctx, client, input)
	if err != nil {
		return err
	}
	asset, err := client.ReadAsset(ctx, input.AssetID)
	if err != nil {
		return fmt.Errorf("read asset failed: %w", err)
	}

	outputs, err := listLegacyMultisigOutputs(ctx, client, senders, senderThreshold, input.AssetID, "")
	if err != nil {
		return fmt.Errorf("list multisig outputs failed: %w", err)
	}
	raw, state, err := findLegacyMultisigTransactionInOutputs(ctx, client, outputs, input, senders, senderThreshold, receiver)
	if err != nil {
		return err
	}
	if raw == "" {
		outputs, err = selectLegacyMultisigOutputs(outputs, input.Amount)
		if err != nil {
			return err
		}
		if err := validateLegacySourceOutputs(outputs, senders, senderThreshold, input.AssetID); err != nil {
			return err
		}

		builder := mixin.NewLegacyTransactionBuilder(outputs)
		builder.Hint = input.TraceID
		builder.Memo = input.Memo
		tx, err := client.MakeTransaction(ctx, builder, []*mixin.TransactionOutput{{
			Address: receiver,
			Amount:  input.Amount,
		}})
		if err != nil {
			return fmt.Errorf("make multisig transaction failed: %w", err)
		}
		payload, err := tx.DumpPayload()
		if err != nil {
			return fmt.Errorf("dump multisig transaction failed: %w", err)
		}
		raw = hex.EncodeToString(payload)
		state = mixin.UTXOStateUnspent
	}

	cmd.Printf("Transfer %s %s from %d/%d multisig to %s\n", input.Amount, asset.Symbol, senderThreshold, len(senders), receiverNames)
	cmd.Println("trace id:", input.TraceID)
	cmd.Println("raw transaction:", raw)
	if state == mixin.UTXOStateSpent {
		cmd.Println("transaction already completed")
		return nil
	}
	if !yes && !conformTransfer() {
		return nil
	}

	request, err := client.CreateMultisig(ctx, mixin.MultisigActionSign, raw)
	if err != nil {
		return fmt.Errorf("create multisig request failed: %w", err)
	}
	if err := validateLegacyMultisigRequest(request, input, senders, senderThreshold); err != nil {
		return fmt.Errorf("validate multisig request failed: %w", err)
	}

	currentID := client.ClientID
	if len(request.Signers) >= int(request.Threshold) {
		cmd.Println("signature threshold already reached")
	} else if !containsString(request.Signers, currentID) {
		pin, err := cmdutil.GetOrReadPin(session.From(ctx))
		if err != nil {
			return fmt.Errorf("read pin failed: %w", err)
		}
		request, err = client.SignMultisig(ctx, request.RequestID, pin)
		if err != nil {
			return fmt.Errorf("sign multisig request failed: %w", err)
		}
	} else {
		cmd.Println("signature already exists for current user")
	}

	printJSON(cmd, request)
	if len(request.Signers) < int(request.Threshold) {
		cmd.Printf("signatures: %d/%d\n", len(request.Signers), request.Threshold)
		return nil
	}
	if request.RawTransaction == "" {
		return errors.New("signed multisig request has no raw transaction")
	}

	tx, err := mixinnet.NewClient(mixinnet.DefaultLegacyConfig).SendRawTransaction(ctx, request.RawTransaction)
	if err != nil {
		return fmt.Errorf("broadcast multisig transaction failed: %w", err)
	}
	cmd.Println("transaction hash:", tx.Hash)
	return nil
}

func validateLegacyMultisigTransfer(input mixin.TransferInput, senders []string, senderThreshold uint8) error {
	if input.TraceID == "" {
		return errors.New("trace is required for multisig transfers")
	}
	if input.AssetID == "" {
		return errors.New("asset is required")
	}
	if !input.Amount.IsPositive() {
		return errors.New("amount must be positive")
	}
	if !input.Amount.Equal(input.Amount.Truncate(mixinnet.Precision)) {
		return fmt.Errorf("amount supports at most %d decimal places", mixinnet.Precision)
	}
	if len(input.Memo) > mixinnet.ExtraSizeGeneralLimit {
		return fmt.Errorf("memo exceeds %d bytes", mixinnet.ExtraSizeGeneralLimit)
	}
	if err := validateMultisigMembers(senders, senderThreshold, "senders"); err != nil {
		return err
	}
	return validateLegacyDestination(input)
}

func validateLegacyDestination(input mixin.TransferInput) error {
	receivers := input.OpponentMultisig.Receivers
	if input.OpponentID != "" && len(receivers) > 0 {
		return errors.New("opponent and receivers are mutually exclusive")
	}
	if input.OpponentID == "" && len(receivers) == 0 {
		return errors.New("opponent or receivers is required")
	}
	if len(receivers) > 0 {
		return validateMultisigMembers(receivers, input.OpponentMultisig.Threshold, "receivers")
	}
	if input.OpponentMultisig.Threshold != 0 {
		return errors.New("receivers are required when threshold is set")
	}
	return nil
}

func validateMultisigMembers(members []string, threshold uint8, name string) error {
	if len(members) == 0 {
		return fmt.Errorf("%s are required", name)
	}
	if threshold == 0 || int(threshold) > len(members) {
		return fmt.Errorf("%s threshold must be in range [1, %s count]", strings.TrimSuffix(name, "s"), name)
	}
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member == "" {
			return fmt.Errorf("%s must not contain an empty member", name)
		}
		if _, ok := seen[member]; ok {
			return fmt.Errorf("%s must not contain duplicate members", name)
		}
		seen[member] = struct{}{}
	}
	return nil
}

func legacyTransferReceiver(ctx context.Context, client legacyMultisigClient, input mixin.TransferInput) (*mixin.MixAddress, []string, error) {
	if err := validateLegacyDestination(input); err != nil {
		return nil, nil, err
	}
	members := input.OpponentMultisig.Receivers
	threshold := input.OpponentMultisig.Threshold
	if len(members) == 0 {
		members = []string{input.OpponentID}
		threshold = 1
	}
	address, err := mixin.NewMixAddress(members, threshold)
	if err != nil {
		return nil, nil, fmt.Errorf("create receiver address failed: %w", err)
	}
	names := make([]string, 0, len(members))
	for _, id := range members {
		user, err := client.ReadUser(ctx, id)
		if err != nil {
			return nil, nil, fmt.Errorf("read receiver %s failed: %w", id, err)
		}
		names = append(names, user.FullName)
	}
	return address, names, nil
}

func listLegacyMultisigOutputs(ctx context.Context, client legacyOutputLister, members []string, threshold uint8, assetID, state string) ([]*mixin.MultisigUTXO, error) {
	const limit = 500
	var (
		result []*mixin.MultisigUTXO
		offset time.Time
	)
	seen := make(map[string]struct{})
	for {
		items, err := client.ListMultisigOutputs(ctx, mixin.ListMultisigOutputsOption{
			Members:        append([]string(nil), members...),
			Threshold:      threshold,
			Offset:         offset,
			Limit:          limit,
			OrderByCreated: true,
			State:          state,
		})
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return result, nil
		}
		for _, item := range items {
			key := item.UTXOID
			if key == "" {
				key = fmt.Sprintf("%s:%d", item.TransactionHash.String(), item.OutputIndex)
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if item.AssetID == assetID {
				result = append(result, item)
			}
		}
		if len(items) < limit {
			return result, nil
		}
		next := items[len(items)-1].CreatedAt
		if !next.After(offset) {
			return nil, fmt.Errorf("legacy output pagination stalled at %s", offset.UTC().Format(time.RFC3339Nano))
		}
		offset = next
	}
}

func selectLegacyMultisigOutputs(outputs []*mixin.MultisigUTXO, amount decimal.Decimal) ([]*mixin.MultisigUTXO, error) {
	balance := decimal.Zero
	selected := make([]*mixin.MultisigUTXO, 0, min(len(outputs), legacyTransactionInputLimit))
	for _, output := range outputs {
		if output.State != "" && output.State != mixin.UTXOStateUnspent {
			continue
		}
		selected = append(selected, output)
		balance = balance.Add(output.Amount)
		if !balance.LessThan(amount) {
			return selected, nil
		}
		if len(selected) == legacyTransactionInputLimit {
			return nil, fmt.Errorf("insufficient balance in first %d outputs; merge outputs before retrying", legacyTransactionInputLimit)
		}
	}
	return nil, fmt.Errorf("insufficient multisig balance: available %s, required %s", balance, amount)
}

func validateLegacySourceOutputs(outputs []*mixin.MultisigUTXO, senders []string, threshold uint8, assetID string) error {
	for i, output := range outputs {
		if output == nil {
			return fmt.Errorf("invalid legacy source output %d: nil", i)
		}
		if output.AssetID != assetID {
			return fmt.Errorf("invalid legacy source output %s: asset mismatch", output.UTXOID)
		}
		if !output.Amount.IsPositive() {
			return fmt.Errorf("invalid legacy source output %s: non-positive amount", output.UTXOID)
		}
		if output.OutputIndex < 0 || output.OutputIndex > 255 {
			return fmt.Errorf("invalid legacy source output %s: output index %d", output.UTXOID, output.OutputIndex)
		}
		if output.Threshold != threshold || !sameMembers(output.Members, senders) {
			return fmt.Errorf("invalid legacy source output %s: multisig group mismatch", output.UTXOID)
		}
		if _, err := mixin.NewMixAddress(output.Members, output.Threshold); err != nil {
			return fmt.Errorf("invalid legacy source output %s: %w", output.UTXOID, err)
		}
	}
	return nil
}

func findLegacyMultisigTransaction(ctx context.Context, client legacyMultisigClient, input mixin.TransferInput, senders []string, senderThreshold uint8, receiver *mixin.MixAddress) (string, string, error) {
	outputs, err := listLegacyMultisigOutputs(ctx, client, senders, senderThreshold, input.AssetID, "")
	if err != nil {
		return "", "", fmt.Errorf("list multisig outputs failed: %w", err)
	}
	return findLegacyMultisigTransactionInOutputs(ctx, client, outputs, input, senders, senderThreshold, receiver)
}

func findLegacyMultisigTransactionInOutputs(ctx context.Context, client legacyTransactionMaker, outputs []*mixin.MultisigUTXO, input mixin.TransferInput, senders []string, senderThreshold uint8, receiver *mixin.MixAddress) (string, string, error) {
	groups := groupLegacyOutputsByRaw(outputs)
	raws := make([]string, 0, len(groups))
	for raw := range groups {
		raws = append(raws, raw)
	}
	sort.Strings(raws)
	for _, raw := range raws {
		matched, err := legacyRawMatchesTransfer(ctx, client, raw, groups[raw], input, senders, senderThreshold, receiver)
		if err != nil {
			return "", "", err
		}
		if matched {
			return raw, groups[raw][0].State, nil
		}
	}
	return "", "", nil
}

func groupLegacyOutputsByRaw(outputs []*mixin.MultisigUTXO) map[string][]*mixin.MultisigUTXO {
	groups := make(map[string][]*mixin.MultisigUTXO)
	for _, output := range outputs {
		if output.SignedTx != "" {
			groups[output.SignedTx] = append(groups[output.SignedTx], output)
		}
	}
	return groups
}

func legacyRawMatchesTransfer(ctx context.Context, client legacyTransactionMaker, raw string, outputs []*mixin.MultisigUTXO, input mixin.TransferInput, senders []string, senderThreshold uint8, receiver *mixin.MixAddress) (bool, error) {
	tx, err := mixinnet.TransactionFromRaw(raw)
	if err != nil {
		return false, nil
	}
	if string(tx.Extra) != input.Memo || len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return false, nil
	}
	outputAmount, err := decimal.NewFromString(tx.Outputs[0].Amount.String())
	if err != nil || !outputAmount.Equal(input.Amount) {
		return false, nil
	}

	byInput := make(map[string]*mixin.MultisigUTXO, len(outputs))
	for _, output := range outputs {
		byInput[fmt.Sprintf("%s:%d", output.TransactionHash.String(), output.OutputIndex)] = output
	}
	inputs := make([]*mixin.MultisigUTXO, 0, len(tx.Inputs))
	for _, txInput := range tx.Inputs {
		if txInput.Hash == nil {
			return false, nil
		}
		output := byInput[fmt.Sprintf("%s:%d", txInput.Hash.String(), txInput.Index)]
		if output == nil {
			return false, nil
		}
		inputs = append(inputs, output)
	}
	if err := validateLegacySourceOutputs(inputs, senders, senderThreshold, input.AssetID); err != nil {
		return false, nil
	}

	builder := mixin.NewLegacyTransactionBuilder(inputs)
	builder.Hint = input.TraceID
	builder.Memo = input.Memo
	expected, err := client.MakeTransaction(ctx, builder, []*mixin.TransactionOutput{{Address: receiver, Amount: input.Amount}})
	if err != nil {
		return false, fmt.Errorf("rebuild legacy multisig transaction failed: %w", err)
	}
	expectedPayload, err := expected.DumpPayload()
	if err != nil {
		return false, fmt.Errorf("dump expected transaction failed: %w", err)
	}
	actualPayload, err := tx.DumpPayload()
	if err != nil {
		return false, fmt.Errorf("dump existing transaction failed: %w", err)
	}
	return hex.EncodeToString(expectedPayload) == hex.EncodeToString(actualPayload), nil
}

func validateLegacyMultisigRequest(request *mixin.MultisigRequest, input mixin.TransferInput, senders []string, senderThreshold uint8) error {
	if request.AssetID != input.AssetID {
		return fmt.Errorf("asset mismatch: expected %s, got %s", input.AssetID, request.AssetID)
	}
	if !request.Amount.Equal(input.Amount) {
		return fmt.Errorf("amount mismatch: expected %s, got %s", input.Amount, request.Amount)
	}
	if request.Memo != input.Memo {
		return fmt.Errorf("memo mismatch: expected %q, got %q", input.Memo, request.Memo)
	}
	if request.Threshold != senderThreshold || !sameMembers(request.Senders, senders) {
		return errors.New("source multisig members or threshold mismatch")
	}
	wantReceivers := input.OpponentMultisig.Receivers
	if len(wantReceivers) == 0 {
		wantReceivers = []string{input.OpponentID}
	}
	if !sameMembers(request.Receivers, wantReceivers) {
		return errors.New("receiver mismatch")
	}
	return nil
}

func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func printJSON(cmd *cobra.Command, value interface{}) {
	data, _ := json.MarshalIndent(value, "", "  ")
	cmd.Println(string(data))
}

func newCmdCancelMultisigSignature() *cobra.Command {
	var opt struct {
		input           mixin.TransferInput
		amount          string
		senders         []string
		senderThreshold uint8
		raw             string
		yes             bool
	}
	cmd := &cobra.Command{
		Use:     "cancel",
		Aliases: []string{"unlock", "cancel-signature"},
		Short:   "cancel the current user's signature on a legacy multisig transfer",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := session.From(ctx).GetClient()
			if err != nil {
				return err
			}

			raw := opt.raw
			if raw == "" {
				opt.input.Amount, _ = decimal.NewFromString(opt.amount)
				if err := validateLegacyMultisigTransfer(opt.input, opt.senders, opt.senderThreshold); err != nil {
					return err
				}
				receiver, _, err := legacyTransferReceiver(ctx, client, opt.input)
				if err != nil {
					return err
				}
				raw, _, err = findLegacyMultisigTransaction(ctx, client, opt.input, opt.senders, opt.senderThreshold, receiver)
				if err != nil {
					return err
				}
				if raw == "" {
					return errors.New("multisig transfer not found")
				}
			}

			signRequest, err := client.CreateMultisig(ctx, mixin.MultisigActionSign, raw)
			if err != nil {
				return fmt.Errorf("read multisig signatures failed: %w", err)
			}
			if !containsString(signRequest.Signers, client.ClientID) {
				cmd.Println("signature already absent for current user")
				return nil
			}
			if len(signRequest.Signers) >= int(signRequest.Threshold) {
				return errors.New("cannot cancel a completed multisig transfer")
			}
			if !opt.yes && !conformTransfer() {
				return nil
			}
			unlockRequest, err := client.CreateMultisig(ctx, mixin.MultisigActionUnlock, raw)
			if err != nil {
				return fmt.Errorf("create unlock request failed: %w", err)
			}
			pin, err := cmdutil.GetOrReadPin(session.From(ctx))
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}
			if err := client.UnlockMultisig(ctx, unlockRequest.RequestID, pin); err != nil {
				return fmt.Errorf("cancel multisig signature failed: %w", err)
			}
			cmd.Println("signature canceled:", unlockRequest.RequestID)
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
	cmd.Flags().StringSliceVar(&opt.senders, "senders", nil, "source multisig members")
	cmd.Flags().Uint8Var(&opt.senderThreshold, "sender-threshold", 0, "source multisig threshold")
	cmd.Flags().StringVar(&opt.raw, "raw", "", "existing signed raw transaction")
	cmd.Flags().BoolVar(&opt.yes, "yes", false, "cancel signature without confirmation")
	return cmd
}

func newCmdCancelMultisigRequest() *cobra.Command {
	var requestID string
	var yes bool
	cmd := &cobra.Command{
		Use:   "cancel-request",
		Short: "cancel an unsigned legacy multisig action request",
		RunE: func(cmd *cobra.Command, args []string) error {
			if requestID == "" {
				return errors.New("request is required")
			}
			if !yes && !conformTransfer() {
				return nil
			}
			client, err := session.From(cmd.Context()).GetClient()
			if err != nil {
				return err
			}
			if err := client.CancelMultisig(cmd.Context(), requestID); err != nil {
				return fmt.Errorf("cancel multisig request failed: %w", err)
			}
			cmd.Println("request canceled:", requestID)
			return nil
		},
	}
	cmd.Flags().StringVar(&requestID, "request", "", "multisig request id")
	cmd.Flags().BoolVar(&yes, "yes", false, "cancel request without confirmation")
	return cmd
}
