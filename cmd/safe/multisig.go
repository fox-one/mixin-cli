package safe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/fox-one/pkg/uuid"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

const safeTransactionInputLimit = 256

type safeMultisigRequestDetails struct {
	mixin.SafeMultisigRequest
	Receivers []*mixin.SafeTransactionReceiver `json:"receivers,omitempty"`
}

type safeMultisigSigner interface {
	SafeSignMultisigRequest(context.Context, *mixin.SafeTransactionRequestInput) (*mixin.SafeMultisigRequest, error)
}

func readSafeMultisigRequest(ctx context.Context, client *mixin.Client, idOrHash string) (*safeMultisigRequestDetails, error) {
	var request safeMultisigRequestDetails
	if err := client.Get(ctx, "/safe/multisigs/"+idOrHash, nil, &request); err != nil {
		return nil, err
	}
	return &request, nil
}

func createSafeMultisigTransfer(cmd *cobra.Command, client *mixin.Client, input mixin.TransferInput, opt safeTransferOptions) error {
	ctx := cmd.Context()
	if err := validateSafeMultisigTransfer(input, opt.senders, opt.senderThreshold); err != nil {
		return err
	}
	if !containsSafeMember(opt.senders, client.ClientID) {
		return errors.New("current user is not a source multisig member")
	}

	asset, err := client.SafeReadAsset(ctx, input.AssetID)
	if err != nil {
		return fmt.Errorf("read asset failed: %w", err)
	}
	kernelAsset, err := mixinnet.HashFromString(asset.KernelAssetID)
	if err != nil {
		return fmt.Errorf("invalid kernel asset id: %w", err)
	}
	receiver, receiverNames, err := safeTransferReceiver(ctx, client, input)
	if err != nil {
		return err
	}
	outputs, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
		Members:   append([]string(nil), opt.senders...),
		Threshold: opt.senderThreshold,
		State:     mixin.SafeUtxoStateUnspent,
		Asset:     asset.KernelAssetID,
		Limit:     safeTransactionInputLimit,
		Order:     "ASC",
	})
	if err != nil {
		return fmt.Errorf("list unspent multisig outputs failed: %w", err)
	}
	outputs, err = selectSafeMultisigOutputs(outputs, input.Amount)
	if err != nil {
		return err
	}
	if err := validateSafeSourceOutputs(outputs, opt.senders, opt.senderThreshold, kernelAsset); err != nil {
		return err
	}

	builder := mixin.NewSafeTransactionBuilder(outputs)
	builder.Hint = input.TraceID
	builder.Memo = input.Memo
	tx, err := client.MakeTransaction(ctx, builder, []*mixin.TransactionOutput{{
		Address: receiver,
		Amount:  input.Amount,
	}})
	if err != nil {
		return fmt.Errorf("make safe multisig transaction failed: %w", err)
	}
	raw, err := tx.Dump()
	if err != nil {
		return fmt.Errorf("dump safe multisig transaction failed: %w", err)
	}

	cmd.Printf("Transfer %s %s from %d/%d safe multisig to %s\n", input.Amount, asset.Symbol, opt.senderThreshold, len(opt.senders), receiverNames)
	cmd.Println("trace id:", input.TraceID)
	cmd.Println("raw transaction:", raw)
	if !opt.yes && !conformContinue() {
		return nil
	}

	spend, err := cmdutil.GetOrSpendKey(session.From(ctx))
	if err != nil {
		return fmt.Errorf("read spend key failed: %w", err)
	}
	_, createErr := client.SafeCreateMultisigRequests(ctx, []*mixin.SafeTransactionRequestInput{{
		RequestID:      input.TraceID,
		RawTransaction: raw,
	}})
	// Always read the canonical request back. This validates receiver details that
	// the SDK create response omits and makes concurrent creators converge by trace.
	details, readErr := readSafeMultisigRequest(ctx, client, input.TraceID)
	if readErr != nil {
		if createErr != nil {
			return fmt.Errorf("create safe multisig request failed: %w", createErr)
		}
		return fmt.Errorf("read created safe multisig request failed: %w", readErr)
	}
	if validateErr := validateExistingSafeMultisigRequest(cmd, details, opt); validateErr != nil {
		return fmt.Errorf("safe multisig trace conflict: %w", validateErr)
	}
	request := &details.SafeMultisigRequest
	if err := validateSafeMultisigRequest(request, input, opt.senders, opt.senderThreshold); err != nil {
		return fmt.Errorf("validate safe multisig request failed: %w", err)
	}
	if err := validateSafeRequestRaw(request.RawTransaction, raw); err != nil {
		return fmt.Errorf("safe multisig trace conflict: %w", err)
	}
	request, err = signSafeMultisigRequest(ctx, client, client.ClientID, request, spend)
	if err != nil {
		return err
	}
	printSafeMultisigRequest(cmd, request)
	cmd.Printf("signatures: %d/%d\n", len(request.Signers), request.SendersThreshold)
	if request.TransactionHash != "" {
		cmd.Println("transaction hash:", request.TransactionHash)
	}
	return nil
}

func continueSafeMultisigTransfer(cmd *cobra.Command, client *mixin.Client, details *safeMultisigRequestDetails, opt safeTransferOptions) error {
	request := &details.SafeMultisigRequest
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err != nil {
		return err
	}
	if !containsSafeMember(request.Senders, client.ClientID) {
		return errors.New("current user is not a source multisig member")
	}

	tx, err := mixinnet.TransactionFromRaw(request.RawTransaction)
	if err != nil {
		return fmt.Errorf("parse safe multisig transaction failed: %w", err)
	}
	if err := validateSafeRequestTransaction(request, tx); err != nil {
		return err
	}
	cmd.Printf("Continue safe multisig transfer %s %s (%d/%d signatures)\n", request.Amount, request.AssetID, len(request.Signers), request.SendersThreshold)
	printSafeMultisigDetails(cmd, details)
	printSafeJSON(cmd, tx)
	cmd.Println("raw transaction:", request.RawTransaction)

	if containsSafeMember(request.Signers, client.ClientID) {
		cmd.Println("signature already exists for current user")
		printSafeMultisigRequest(cmd, request)
		return nil
	}
	if len(request.Signers) >= int(request.SendersThreshold) {
		cmd.Println("transaction already completed")
		printSafeMultisigRequest(cmd, request)
		return nil
	}
	if !opt.yes && !conformContinue() {
		return nil
	}

	spend, err := cmdutil.GetOrSpendKey(session.From(cmd.Context()))
	if err != nil {
		return fmt.Errorf("read spend key failed: %w", err)
	}
	request, err = signSafeMultisigRequest(cmd.Context(), client, client.ClientID, request, spend)
	if err != nil {
		return err
	}
	printSafeMultisigRequest(cmd, request)
	cmd.Printf("signatures: %d/%d\n", len(request.Signers), request.SendersThreshold)
	if request.TransactionHash != "" {
		cmd.Println("transaction hash:", request.TransactionHash)
	}
	return nil
}

func signSafeMultisigRequest(ctx context.Context, client safeMultisigSigner, currentID string, request *mixin.SafeMultisigRequest, spend *mixinnet.Key) (*mixin.SafeMultisigRequest, error) {
	if request == nil {
		return nil, errors.New("invalid safe multisig request: empty request")
	}
	if spend == nil {
		return nil, errors.New("safe multisig spend key is required")
	}
	if err := validateSafeMembers(request.Senders, request.SendersThreshold, "senders"); err != nil {
		return nil, fmt.Errorf("invalid safe multisig request: %w", err)
	}
	if containsSafeMember(request.Signers, currentID) {
		return request, nil
	}
	if len(request.Signers) >= int(request.SendersThreshold) {
		return request, nil
	}
	index, err := safeSignerIndex(request.Senders, currentID)
	if err != nil {
		return nil, err
	}
	tx, err := mixinnet.TransactionFromRaw(request.RawTransaction)
	if err != nil {
		return nil, fmt.Errorf("parse safe multisig transaction failed: %w", err)
	}
	if err := validateSafeRequestTransaction(request, tx); err != nil {
		return nil, err
	}
	if len(request.Views) != len(tx.Inputs) {
		return nil, fmt.Errorf("invalid safe multisig views: got %d for %d inputs", len(request.Views), len(tx.Inputs))
	}
	if tx.Signatures != nil && len(tx.Signatures) != len(tx.Inputs) {
		return nil, fmt.Errorf("invalid safe multisig signatures: got %d maps for %d inputs", len(tx.Signatures), len(tx.Inputs))
	}
	for i, view := range request.Views {
		if _, err := view.ToScalar(); err != nil {
			return nil, fmt.Errorf("invalid safe multisig view %d: %w", i, err)
		}
	}
	if err := mixin.SafeSignTransaction(tx, *spend, request.Views, index); err != nil {
		return nil, fmt.Errorf("sign safe multisig transaction failed: %w", err)
	}
	raw, err := tx.Dump()
	if err != nil {
		return nil, fmt.Errorf("dump signed safe multisig transaction failed: %w", err)
	}
	signedRequest, err := client.SafeSignMultisigRequest(ctx, &mixin.SafeTransactionRequestInput{
		RequestID:      request.RequestID,
		RawTransaction: raw,
	})
	if err != nil {
		return nil, fmt.Errorf("submit safe multisig signature failed: %w", err)
	}
	if err := validateSafeSignedResponse(request, signedRequest, raw, currentID); err != nil {
		return nil, fmt.Errorf("validate signed safe multisig request failed: %w", err)
	}
	return signedRequest, nil
}

func validateSafeMultisigTransfer(input mixin.TransferInput, senders []string, threshold uint8) error {
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
	if err := validateSafeMembers(senders, threshold, "senders"); err != nil {
		return err
	}
	return validateSafeDestination(input)
}

func validateSafeDestination(input mixin.TransferInput) error {
	receivers := input.OpponentMultisig.Receivers
	if input.OpponentID != "" && len(receivers) > 0 {
		return errors.New("opponent and receivers are mutually exclusive")
	}
	if input.OpponentID == "" && len(receivers) == 0 {
		return errors.New("opponent or receivers is required")
	}
	if len(receivers) > 0 {
		return validateSafeMembers(receivers, input.OpponentMultisig.Threshold, "receivers")
	}
	if input.OpponentMultisig.Threshold != 0 {
		return errors.New("receivers are required when threshold is set")
	}
	return nil
}

func validateSafeMembers(members []string, threshold uint8, name string) error {
	if len(members) == 0 {
		return fmt.Errorf("%s are required", name)
	}
	if threshold == 0 || int(threshold) > len(members) {
		return fmt.Errorf("%s threshold must be in range [1, %s count]", name, name)
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

func safeTransferReceiver(ctx context.Context, client *mixin.Client, input mixin.TransferInput) (*mixin.MixAddress, []string, error) {
	if err := validateSafeDestination(input); err != nil {
		return nil, nil, err
	}
	members := input.OpponentMultisig.Receivers
	threshold := input.OpponentMultisig.Threshold
	if len(members) == 0 {
		members = []string{input.OpponentID}
		threshold = 1
	}

	names := make([]string, 0, len(members))
	if _, err := uuid.FromString(members[0]); err != nil {
		address, err := mixin.NewMainnetMixAddress(members, threshold)
		if err != nil {
			return nil, nil, fmt.Errorf("create mainnet receiver address failed: %w", err)
		}
		return address, append(names, members...), nil
	}
	address, err := mixin.NewMixAddress(members, threshold)
	if err != nil {
		return nil, nil, fmt.Errorf("create receiver address failed: %w", err)
	}
	for _, id := range members {
		user, err := client.ReadUser(ctx, id)
		if err != nil {
			return nil, nil, fmt.Errorf("read receiver %s failed: %w", id, err)
		}
		names = append(names, user.FullName)
	}
	return address, names, nil
}

func selectSafeMultisigOutputs(outputs []*mixin.SafeUtxo, amount decimal.Decimal) ([]*mixin.SafeUtxo, error) {
	balance := decimal.Zero
	selected := make([]*mixin.SafeUtxo, 0, min(len(outputs), safeTransactionInputLimit))
	for _, output := range outputs {
		if output == nil {
			return nil, errors.New("invalid safe source output: nil")
		}
		if output.State != "" && output.State != mixin.SafeUtxoStateUnspent {
			continue
		}
		if !output.Amount.IsPositive() {
			return nil, fmt.Errorf("invalid safe source output %s: non-positive amount", output.OutputID)
		}
		selected = append(selected, output)
		balance = balance.Add(output.Amount)
		if !balance.LessThan(amount) {
			return selected, nil
		}
		if len(selected) == safeTransactionInputLimit {
			return nil, fmt.Errorf("insufficient balance in first %d outputs; merge outputs before retrying", safeTransactionInputLimit)
		}
	}
	return nil, fmt.Errorf("insufficient safe multisig balance: available %s, required %s", balance, amount)
}

func validateSafeSourceOutputs(outputs []*mixin.SafeUtxo, senders []string, threshold uint8, asset mixinnet.Hash) error {
	var sourceAddress string
	for i, output := range outputs {
		if output == nil {
			return fmt.Errorf("invalid safe source output %d: nil", i)
		}
		if output.KernelAssetID != asset {
			return fmt.Errorf("invalid safe source output %s: asset mismatch", output.OutputID)
		}
		if !output.Amount.IsPositive() {
			return fmt.Errorf("invalid safe source output %s: non-positive amount", output.OutputID)
		}
		if output.State != "" && output.State != mixin.SafeUtxoStateUnspent {
			return fmt.Errorf("invalid safe source output %s: state %s", output.OutputID, output.State)
		}
		if output.ReceiversThreshold != threshold || !sameSafeMembers(output.Receivers, senders) {
			return fmt.Errorf("invalid safe source output %s: multisig group mismatch", output.OutputID)
		}
		address, err := mixin.NewMixAddress(output.Receivers, output.ReceiversThreshold)
		if err != nil {
			return fmt.Errorf("invalid safe source output %s: %w", output.OutputID, err)
		}
		if sourceAddress == "" {
			sourceAddress = address.String()
		} else if sourceAddress != address.String() {
			return fmt.Errorf("invalid safe source output %s: inconsistent member order", output.OutputID)
		}
	}
	return nil
}

func validateSafeMultisigRequest(request *mixin.SafeMultisigRequest, input mixin.TransferInput, senders []string, threshold uint8) error {
	if request == nil {
		return errors.New("empty safe multisig request")
	}
	if request.RequestID != input.TraceID {
		return fmt.Errorf("trace mismatch: expected %s, got %s", input.TraceID, request.RequestID)
	}
	if request.AssetID != input.AssetID {
		return fmt.Errorf("asset mismatch: expected %s, got %s", input.AssetID, request.AssetID)
	}
	if !request.Amount.Equal(input.Amount) {
		return fmt.Errorf("amount mismatch: expected %s, got %s", input.Amount, request.Amount)
	}
	if request.Extra != input.Memo {
		return fmt.Errorf("memo mismatch: expected %q, got %q", input.Memo, request.Extra)
	}
	if request.SendersThreshold != threshold || !sameSafeMembers(request.Senders, senders) {
		return errors.New("source multisig members or threshold mismatch")
	}
	if err := validateSafeRequestSigners(request); err != nil {
		return err
	}
	return nil
}

func validateExistingSafeMultisigRequest(cmd *cobra.Command, details *safeMultisigRequestDetails, opt safeTransferOptions) error {
	request := &details.SafeMultisigRequest
	if request.RequestID == "" || request.RawTransaction == "" {
		return errors.New("invalid safe multisig request")
	}
	if request.RequestID != opt.input.TraceID && request.TransactionHash != opt.input.TraceID {
		return fmt.Errorf("safe multisig request does not match trace %s", opt.input.TraceID)
	}
	if err := validateSafeMembers(request.Senders, request.SendersThreshold, "senders"); err != nil {
		return fmt.Errorf("invalid safe multisig request: %w", err)
	}
	if err := validateSafeRequestSigners(request); err != nil {
		return err
	}
	if opt.input.AssetID != "" && request.AssetID != opt.input.AssetID {
		return fmt.Errorf("asset mismatch: expected %s, got %s", opt.input.AssetID, request.AssetID)
	}
	if opt.amount != "" {
		amount, err := decimal.NewFromString(opt.amount)
		if err != nil || !amount.IsPositive() {
			return errors.New("amount must be positive")
		}
		if !request.Amount.Equal(amount) {
			return fmt.Errorf("amount mismatch: expected %s, got %s", amount, request.Amount)
		}
	}
	if cmd.Flags().Changed("memo") && request.Extra != opt.input.Memo {
		return fmt.Errorf("memo mismatch: expected %q, got %q", opt.input.Memo, request.Extra)
	}
	if opt.isMultisigSource() {
		if err := validateSafeMembers(opt.senders, opt.senderThreshold, "senders"); err != nil {
			return err
		}
		if request.SendersThreshold != opt.senderThreshold || !sameSafeMembers(request.Senders, opt.senders) {
			return errors.New("source multisig members or threshold mismatch")
		}
	}
	if opt.input.OpponentID != "" || len(opt.input.OpponentMultisig.Receivers) > 0 {
		if err := validateSafeDestination(opt.input); err != nil {
			return err
		}
		if len(details.Receivers) == 0 {
			return errors.New("safe multisig request has no receiver details")
		}
		members := opt.input.OpponentMultisig.Receivers
		threshold := opt.input.OpponentMultisig.Threshold
		if len(members) == 0 {
			members = []string{opt.input.OpponentID}
			threshold = 1
		}
		if details.Receivers[0].Threshold != threshold || !sameSafeMembers(details.Receivers[0].Members, members) {
			return errors.New("safe multisig receiver mismatch")
		}
	}
	return nil
}

func validateSafeRequestSigners(request *mixin.SafeMultisigRequest) error {
	seen := make(map[string]struct{}, len(request.Signers))
	for _, signer := range request.Signers {
		if !containsSafeMember(request.Senders, signer) {
			return fmt.Errorf("invalid safe multisig signer %s: not a sender", signer)
		}
		if _, ok := seen[signer]; ok {
			return fmt.Errorf("invalid safe multisig signer %s: duplicate", signer)
		}
		seen[signer] = struct{}{}
	}
	return nil
}

func validateSafeRequestTransaction(request *mixin.SafeMultisigRequest, tx *mixinnet.Transaction) error {
	if request == nil {
		return errors.New("invalid safe multisig request: empty request")
	}
	if tx == nil || len(tx.Inputs) == 0 || len(tx.Outputs) == 0 {
		return errors.New("invalid safe multisig transaction: inputs and outputs are required")
	}
	if tx.Asset != request.KernelAssetID {
		return errors.New("safe multisig raw transaction asset mismatch")
	}
	if string(tx.Extra) != request.Extra {
		return errors.New("safe multisig raw transaction memo mismatch")
	}
	amount, err := decimal.NewFromString(tx.Outputs[0].Amount.String())
	if err != nil || !amount.Equal(request.Amount) {
		return errors.New("safe multisig raw transaction amount mismatch")
	}
	if request.TransactionHash != "" {
		hash, err := tx.TransactionHash()
		if err != nil {
			return fmt.Errorf("hash safe multisig raw transaction failed: %w", err)
		}
		if hash.String() != request.TransactionHash {
			return errors.New("safe multisig raw transaction hash mismatch")
		}
	}
	return nil
}

func validateSafeRequestRaw(candidateRaw, confirmedRaw string) error {
	candidate, err := mixinnet.TransactionFromRaw(candidateRaw)
	if err != nil {
		return fmt.Errorf("parse request raw transaction failed: %w", err)
	}
	confirmed, err := mixinnet.TransactionFromRaw(confirmedRaw)
	if err != nil {
		return fmt.Errorf("parse confirmed raw transaction failed: %w", err)
	}
	candidatePayload, err := candidate.DumpPayload()
	if err != nil {
		return fmt.Errorf("dump request transaction payload failed: %w", err)
	}
	confirmedPayload, err := confirmed.DumpPayload()
	if err != nil {
		return fmt.Errorf("dump confirmed transaction payload failed: %w", err)
	}
	if !bytes.Equal(candidatePayload, confirmedPayload) {
		return errors.New("raw transaction payload does not match the confirmed transfer")
	}
	return nil
}

func validateSafeSignedResponse(previous, signed *mixin.SafeMultisigRequest, submittedRaw, currentID string) error {
	if signed == nil {
		return errors.New("empty safe multisig request")
	}
	if signed.RequestID != previous.RequestID {
		return fmt.Errorf("request id mismatch: expected %s, got %s", previous.RequestID, signed.RequestID)
	}
	if signed.AssetID != previous.AssetID || signed.KernelAssetID != previous.KernelAssetID || !signed.Amount.Equal(previous.Amount) || signed.Extra != previous.Extra {
		return errors.New("signed safe multisig transfer fields changed")
	}
	if signed.SendersThreshold != previous.SendersThreshold || !sameSafeMembers(signed.Senders, previous.Senders) {
		return errors.New("signed safe multisig source changed")
	}
	if err := validateSafeMembers(signed.Senders, signed.SendersThreshold, "senders"); err != nil {
		return fmt.Errorf("invalid signed safe multisig request: %w", err)
	}
	if err := validateSafeRequestSigners(signed); err != nil {
		return err
	}
	for _, signer := range previous.Signers {
		if !containsSafeMember(signed.Signers, signer) {
			return fmt.Errorf("existing safe multisig signer %s disappeared", signer)
		}
	}
	if !containsSafeMember(signed.Signers, currentID) {
		return errors.New("signed safe multisig response is missing the current signer")
	}
	if signed.RawTransaction == "" {
		return errors.New("signed safe multisig request has no raw transaction")
	}
	if err := validateSafeRequestRaw(signed.RawTransaction, submittedRaw); err != nil {
		return err
	}
	tx, err := mixinnet.TransactionFromRaw(signed.RawTransaction)
	if err != nil {
		return fmt.Errorf("parse signed safe multisig transaction failed: %w", err)
	}
	return validateSafeRequestTransaction(signed, tx)
}

func safeSignerIndex(members []string, clientID string) (uint16, error) {
	if len(members) == 0 {
		return 0, errors.New("safe multisig request has no senders")
	}
	members = append([]string(nil), members...)
	sort.Strings(members)
	for i, member := range members {
		if (i > 0 && member == members[i-1]) || (i+1 < len(members) && member == members[i+1]) {
			return 0, errors.New("safe multisig request has duplicate senders")
		}
		if member == clientID {
			return uint16(i), nil
		}
	}
	return 0, errors.New("current user is not a source multisig member")
}

func sameSafeMembers(a, b []string) bool {
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

func containsSafeMember(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func printSafeJSON(cmd *cobra.Command, value interface{}) {
	data, _ := json.MarshalIndent(value, "", "  ")
	cmd.Println(string(data))
}

func printSafeMultisigRequest(cmd *cobra.Command, request *mixin.SafeMultisigRequest) {
	if request == nil {
		printSafeJSON(cmd, nil)
		return
	}
	redacted := *request
	redacted.Views = nil
	printSafeJSON(cmd, &redacted)
}

func printSafeMultisigDetails(cmd *cobra.Command, details *safeMultisigRequestDetails) {
	if details == nil {
		printSafeJSON(cmd, nil)
		return
	}
	redacted := *details
	redacted.SafeMultisigRequest.Views = nil
	printSafeJSON(cmd, &redacted)
}

func newCmdCancelSafeMultisigSignature() *cobra.Command {
	var trace string
	var yes bool
	cmd := &cobra.Command{
		Use:     "cancel",
		Aliases: []string{"unlock", "cancel-signature"},
		Short:   "cancel the current user's signature on a safe multisig transfer",
		RunE: func(cmd *cobra.Command, args []string) error {
			if trace == "" {
				return errors.New("trace is required")
			}
			client, err := session.From(cmd.Context()).GetClient()
			if err != nil {
				return err
			}
			details, err := readSafeMultisigRequest(cmd.Context(), client, trace)
			if err != nil {
				return fmt.Errorf("read safe multisig request failed: %w", err)
			}
			request := &details.SafeMultisigRequest
			if err := validateSafeMembers(request.Senders, request.SendersThreshold, "senders"); err != nil {
				return fmt.Errorf("invalid safe multisig request: %w", err)
			}
			if err := validateSafeRequestSigners(request); err != nil {
				return err
			}
			tx, err := mixinnet.TransactionFromRaw(request.RawTransaction)
			if err != nil {
				return fmt.Errorf("parse safe multisig transaction failed: %w", err)
			}
			if err := validateSafeRequestTransaction(request, tx); err != nil {
				return err
			}
			if !containsSafeMember(request.Signers, client.ClientID) {
				cmd.Println("signature already absent for current user")
				return nil
			}
			if len(request.Signers) >= int(request.SendersThreshold) {
				return errors.New("cannot cancel a completed multisig transfer")
			}
			printSafeMultisigDetails(cmd, details)
			if !yes && !conformContinue() {
				return nil
			}
			request, err = client.SafeUnlockMultisigRequest(cmd.Context(), request.RequestID)
			if err != nil {
				return fmt.Errorf("cancel safe multisig signature failed: %w", err)
			}
			printSafeMultisigRequest(cmd, request)
			return nil
		},
	}
	cmd.Flags().StringVar(&trace, "trace", "", "safe multisig trace/request id")
	cmd.Flags().BoolVar(&yes, "yes", false, "cancel signature without confirmation")
	return cmd
}
