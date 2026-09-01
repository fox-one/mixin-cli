package safe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func TestSafeSignerIndexUsesCanonicalMemberOrder(t *testing.T) {
	members := []string{"c", "a", "b"}
	index, err := safeSignerIndex(members, "b")
	if err != nil {
		t.Fatal(err)
	}
	if index != 1 {
		t.Fatalf("index = %d, want 1", index)
	}
	if members[0] != "c" {
		t.Fatal("safeSignerIndex mutated the caller's member order")
	}

	if _, err := safeSignerIndex([]string{"a", "a"}, "a"); err == nil {
		t.Fatal("expected duplicate sender error")
	}
	if _, err := safeSignerIndex([]string{"a", "b"}, "c"); err == nil {
		t.Fatal("expected non-member error")
	}
}

func TestSelectSafeMultisigOutputsAcceptsExactBalance(t *testing.T) {
	outputs := []*mixin.SafeUtxo{
		{Amount: decimal.RequireFromString("1.25"), State: mixin.SafeUtxoStateUnspent},
		{Amount: decimal.RequireFromString("2.75"), State: mixin.SafeUtxoStateUnspent},
	}
	selected, err := selectSafeMultisigOutputs(outputs, decimal.NewFromInt(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected %d outputs, want 2", len(selected))
	}
}

func TestSelectSafeMultisigOutputsRejectsNilOutput(t *testing.T) {
	if _, err := selectSafeMultisigOutputs([]*mixin.SafeUtxo{nil}, decimal.NewFromInt(1)); err == nil {
		t.Fatal("expected nil output error")
	}
}

func TestSelectSafeMultisigOutputsSkipsNonUnspent(t *testing.T) {
	outputs := []*mixin.SafeUtxo{
		{Amount: decimal.NewFromInt(10), State: mixin.SafeUtxoStateSigned},
		{Amount: decimal.NewFromInt(2), State: mixin.SafeUtxoStateUnspent},
	}
	if _, err := selectSafeMultisigOutputs(outputs, decimal.NewFromInt(3)); err == nil {
		t.Fatal("expected insufficient balance error")
	}
}

func TestSelectSafeMultisigOutputsUsesLargestAvailableInputs(t *testing.T) {
	outputs := make([]*mixin.SafeUtxo, safeTransactionInputLimit+1)
	for i := 0; i < safeTransactionInputLimit; i++ {
		outputs[i] = &mixin.SafeUtxo{OutputID: fmt.Sprintf("small-%03d", i), Amount: decimal.NewFromInt(1), State: mixin.SafeUtxoStateUnspent}
	}
	outputs[safeTransactionInputLimit] = &mixin.SafeUtxo{OutputID: "large", Amount: decimal.NewFromInt(1000), State: mixin.SafeUtxoStateUnspent}
	selected, err := selectSafeMultisigOutputs(outputs, decimal.NewFromInt(1000))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].OutputID != "large" {
		t.Fatalf("selected %#v, want the single largest output", selected)
	}
}

func TestListSafeMultisigOutputsPaginates(t *testing.T) {
	first := make([]*mixin.SafeUtxo, safeTransactionInputLimit)
	for i := range first {
		first[i] = &mixin.SafeUtxo{OutputID: fmt.Sprintf("output-%03d", i), Sequence: uint64(i)}
	}
	client := &fakeSafeUtxoLister{pages: map[uint64][]*mixin.SafeUtxo{
		0:                         first,
		safeTransactionInputLimit: {{OutputID: "last", Sequence: safeTransactionInputLimit}},
	}}
	outputs, err := listSafeMultisigOutputs(context.Background(), client, []string{"a", "b"}, 2, "asset")
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != safeTransactionInputLimit+1 || len(client.offsets) != 2 || client.offsets[1] != safeTransactionInputLimit {
		t.Fatalf("outputs = %d, offsets = %v", len(outputs), client.offsets)
	}
}

func TestValidateSafeSourceOutputsRejectsWrongGroup(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	outputs := []*mixin.SafeUtxo{{
		OutputID:           "output",
		KernelAssetID:      asset,
		Amount:             decimal.NewFromInt(1),
		Receivers:          []string{"00000000-0000-0000-0000-000000000001"},
		ReceiversThreshold: 1,
	}}
	if err := validateSafeSourceOutputs(outputs, []string{"00000000-0000-0000-0000-000000000002"}, 1, asset); err == nil {
		t.Fatal("expected source group mismatch")
	}
}

func TestValidateSafeSourceOutputsRejectsInconsistentMemberOrder(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	outputs := []*mixin.SafeUtxo{
		{
			OutputID:           "first",
			KernelAssetID:      asset,
			Amount:             decimal.NewFromInt(1),
			Receivers:          []string{members[0], members[1]},
			ReceiversThreshold: 2,
		},
		{
			OutputID:           "second",
			KernelAssetID:      asset,
			Amount:             decimal.NewFromInt(1),
			Receivers:          []string{members[1], members[0]},
			ReceiversThreshold: 2,
		},
	}
	if err := validateSafeSourceOutputs(outputs, members, 2, asset); err == nil {
		t.Fatal("expected inconsistent source address error")
	}
}

func TestValidateExistingSafeMultisigRequest(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("memo", "", "")
	details := &safeMultisigRequestDetails{
		SafeMultisigRequest: mixin.SafeMultisigRequest{
			RequestID:        "trace",
			AssetID:          "asset",
			Amount:           decimal.NewFromInt(3),
			Senders:          []string{"b", "a"},
			SendersThreshold: 2,
			RawTransaction:   "raw",
		},
	}
	opt := safeTransferOptions{}
	opt.input.TraceID = "trace"
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err != nil {
		t.Fatal(err)
	}

	opt.amount = "4"
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err == nil {
		t.Fatal("expected amount mismatch")
	}

	opt.amount = "3"
	opt.senders = []string{"a", "b"}
	opt.senderThreshold = 2
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err != nil {
		t.Fatal(err)
	}

	opt.input.OpponentID = "receiver"
	details.Receivers = []*mixin.SafeTransactionReceiver{{Members: []string{"receiver"}, Threshold: 1}}
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err != nil {
		t.Fatal(err)
	}
	details.Receivers[0].Members = []string{"someone-else"}
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err == nil {
		t.Fatal("expected receiver mismatch")
	}
	details.Receivers = []*mixin.SafeTransactionReceiver{nil}
	if err := validateExistingSafeMultisigRequest(cmd, details, opt); err == nil {
		t.Fatal("expected nil receiver error")
	}
}

func TestSafeMultisigRequestDetailsUnmarshalReceivers(t *testing.T) {
	var details safeMultisigRequestDetails
	data := []byte(`{"request_id":"trace","receivers":[{"members":["receiver"],"threshold":1}]}`)
	if err := json.Unmarshal(data, &details); err != nil {
		t.Fatal(err)
	}
	if details.RequestID != "trace" || len(details.Receivers) != 1 || details.Receivers[0].Members[0] != "receiver" {
		t.Fatalf("unexpected request details: %#v", details)
	}
}

func TestPrintSafeMultisigDetailsRedactsViews(t *testing.T) {
	var view mixinnet.Key
	view[0] = 1
	details := &safeMultisigRequestDetails{
		SafeMultisigRequest: mixin.SafeMultisigRequest{
			RequestID: "trace",
			Views:     []mixinnet.Key{view},
		},
		Receivers: []*mixin.SafeTransactionReceiver{{Members: []string{"receiver"}, Threshold: 1}},
	}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	printSafeMultisigDetails(cmd, details)
	if strings.Contains(output.String(), "\"views\"") {
		t.Fatalf("sensitive views leaked in output: %s", output.String())
	}
	if !strings.Contains(output.String(), "\"request_id\": \"trace\"") || !strings.Contains(output.String(), "\"receivers\"") {
		t.Fatalf("safe request summary lost public fields: %s", output.String())
	}
}

func TestValidateSafeRequestRejectsUnknownSigner(t *testing.T) {
	request := &mixin.SafeMultisigRequest{
		Senders: []string{"a", "b"},
		Signers: []string{"c"},
	}
	if err := validateSafeRequestSigners(request); err == nil {
		t.Fatal("expected unknown signer error")
	}
}

func TestValidateSafeRequestTransaction(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersion,
		Asset:   asset,
		Inputs:  []*mixinnet.Input{{Hash: &inputHash}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(2))}},
		Extra:   []byte("memo"),
	}
	request := &mixin.SafeMultisigRequest{
		KernelAssetID: asset,
		Amount:        decimal.NewFromInt(2),
		Extra:         "memo",
	}
	if err := validateSafeRequestTransaction(request, tx); err != nil {
		t.Fatal(err)
	}

	request.Amount = decimal.NewFromInt(3)
	if err := validateSafeRequestTransaction(request, tx); err == nil {
		t.Fatal("expected raw amount mismatch")
	}
}

func TestValidateSafeRequestDetailsBindsChangeToSource(t *testing.T) {
	tx, raw, request, receivers := safeTransactionFixture(t)
	details := &safeMultisigRequestDetails{
		SafeMultisigRequest: *request,
		Receivers:           receivers,
	}
	if _, err := validateSafeRequestDetails(details); err != nil {
		t.Fatal(err)
	}

	originalChange := details.Receivers[1]
	details.Receivers[1] = &mixin.SafeTransactionReceiver{Members: []string{"attacker"}, Threshold: 1}
	if _, err := validateSafeRequestDetails(details); err == nil {
		t.Fatal("expected malicious change receiver to be rejected")
	}

	details.Receivers[1] = originalChange
	extra := *tx.Outputs[1]
	tx.Outputs = append(tx.Outputs, &extra)
	extraRaw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	details.RawTransaction = extraRaw
	details.Receivers = append(details.Receivers, originalChange)
	if _, err := validateSafeRequestDetails(details); err != nil {
		t.Fatalf("expected split change outputs to be accepted: %v", err)
	}

	details.RawTransaction = raw
	details.Receivers = []*mixin.SafeTransactionReceiver{receivers[0], nil}
	if _, err := validateSafeRequestDetails(details); err == nil {
		t.Fatal("expected nil receiver to be rejected")
	}
}

func TestValidateSafeTransactionResponseBindsLocalTransaction(t *testing.T) {
	tx, raw, _, receivers := safeTransactionFixture(t)
	signature := testSignature(1)
	tx.Signatures = []map[uint16]*mixinnet.Signature{{0: signature}}
	var err error
	raw, err = tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := tx.TransactionHash()
	if err != nil {
		t.Fatal(err)
	}
	input := mixin.TransferInput{
		TraceID:    "trace",
		AssetID:    "asset",
		Amount:     decimal.NewFromInt(2),
		Memo:       "memo",
		OpponentID: "receiver",
	}
	response := &mixin.SafeTransactionRequest{
		RequestID:        input.TraceID,
		TransactionHash:  hash.String(),
		KernelAssetID:    tx.Asset,
		Amount:           input.Amount,
		Extra:            input.Memo,
		Senders:          []string{"sender"},
		SendersThreshold: 1,
		Signers:          []string{"sender"},
		RawTransaction:   raw,
		Receivers:        receivers,
	}
	if err := validateSafeTransactionResponse(response, input, []string{"sender"}, 1, tx, raw, true); err != nil {
		t.Fatal(err)
	}

	conflict := *response
	conflict.RequestID = "other-trace"
	if err := validateSafeTransactionResponse(&conflict, input, []string{"sender"}, 1, tx, raw, true); err == nil {
		t.Fatal("expected request id mismatch")
	}
	conflict = *response
	conflict.TransactionHash = mixinnet.NewHash([]byte("other")).String()
	if err := validateSafeTransactionResponse(&conflict, input, []string{"sender"}, 1, tx, raw, true); err == nil {
		t.Fatal("expected transaction hash mismatch")
	}
	conflict = *response
	conflict.Senders = []string{"attacker"}
	if err := validateSafeTransactionResponse(&conflict, input, []string{"sender"}, 1, tx, raw, true); err == nil {
		t.Fatal("expected source mismatch")
	}
}

func TestValidateSafeUnlockResponseRequiresExactSignerRemoval(t *testing.T) {
	tx, _, request, _ := safeTransactionFixture(t)
	request.Senders = []string{"a", "b", "c"}
	request.SendersThreshold = 3
	request.Signers = []string{"a", "b"}
	tx.Signatures = []map[uint16]*mixinnet.Signature{{0: testSignature(1), 1: testSignature(2)}}
	previousRaw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	request.RawTransaction = previousRaw
	unlocked := *request
	unlocked.Signers = []string{"a"}
	tx.Signatures = []map[uint16]*mixinnet.Signature{{0: testSignature(1)}}
	unlocked.RawTransaction, err = tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSafeUnlockResponse(request, &unlocked, "b"); err != nil {
		t.Fatal(err)
	}
	unlocked.RawTransaction = previousRaw
	if err := validateSafeUnlockResponse(request, &unlocked, "b"); err == nil {
		t.Fatal("expected raw transaction retaining the canceled signature to be rejected")
	}
	tx.Signatures = []map[uint16]*mixinnet.Signature{{0: testSignature(1)}}
	unlocked.RawTransaction, err = tx.Dump()
	if err != nil {
		t.Fatal(err)
	}

	unlocked.Signers = []string{"a", "b"}
	if err := validateSafeUnlockResponse(request, &unlocked, "b"); err == nil {
		t.Fatal("expected current signer to be removed")
	}
	unlocked.Signers = nil
	if err := validateSafeUnlockResponse(request, &unlocked, "b"); err == nil {
		t.Fatal("expected existing signer to remain")
	}
}

func TestSignSafeMultisigRequestUsesCanonicalSignerIndex(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersion,
		Asset:   asset,
		Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	view := mixinnet.GenerateKey(rand.Reader)
	spend := mixinnet.GenerateKey(rand.Reader)
	request := &mixin.SafeMultisigRequest{
		RequestID:        "trace",
		KernelAssetID:    asset,
		Amount:           decimal.NewFromInt(1),
		Senders:          []string{"b", "a"},
		SendersThreshold: 2,
		RawTransaction:   raw,
		Views:            []mixinnet.Key{view},
	}
	response := *request
	response.Signers = []string{"b"}
	client := &fakeSafeMultisigSigner{response: &response}
	if _, err := signSafeMultisigRequest(context.Background(), client, "b", request, &spend); err != nil {
		t.Fatal(err)
	}
	signed, err := mixinnet.TransactionFromRaw(client.raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed.Signatures) != 1 || signed.Signatures[0][1] == nil {
		t.Fatalf("expected signer b at canonical index 1, got %#v", signed.Signatures)
	}
}

func TestValidateSafeSignedResponseRejectsSignerMissingFromRaw(t *testing.T) {
	tx, raw, request, _ := safeTransactionFixture(t)
	request.Senders = []string{"sender"}
	request.SendersThreshold = 1
	tx.Signatures = []map[uint16]*mixinnet.Signature{{0: testSignature(1)}}
	submittedRaw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	signed := *request
	signed.Signers = []string{"sender"}
	signed.RawTransaction = raw
	if err := validateSafeSignedResponse(request, &signed, submittedRaw, "sender"); err == nil {
		t.Fatal("expected signer metadata without a raw signature to be rejected")
	}
}

func TestShouldContinueSafeMultisigTransferRequiresExplicitJoinShape(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("asset", "", "")
	cmd.Flags().String("amount", "", "")
	cmd.Flags().String("memo", "", "")
	cmd.Flags().String("opponent", "", "")
	cmd.Flags().StringSlice("receivers", nil, "")
	cmd.Flags().Uint8("threshold", 0, "")
	opt := safeTransferOptions{input: mixin.TransferInput{TraceID: "trace"}}
	if !shouldContinueSafeMultisigTransfer(cmd, opt) {
		t.Fatal("trace-only invocation should continue an existing multisig request")
	}
	if err := cmd.Flags().Set("amount", "1"); err != nil {
		t.Fatal(err)
	}
	if shouldContinueSafeMultisigTransfer(cmd, opt) {
		t.Fatal("personal transfer arguments must not silently switch to multisig mode")
	}
	opt.senders = []string{"a", "b"}
	opt.senderThreshold = 2
	if !shouldContinueSafeMultisigTransfer(cmd, opt) {
		t.Fatal("an explicit multisig source should continue by trace")
	}
}

func TestReadSafeMultisigRequestUsesSDKReader(t *testing.T) {
	request := &mixin.SafeMultisigRequest{RequestID: "trace"}
	reader := &fakeSafeMultisigReader{request: request}
	got, err := readSafeMultisigRequest(context.Background(), reader, "trace")
	if err != nil {
		t.Fatal(err)
	}
	if got != request {
		t.Fatalf("got %#v, want SDK response %#v", got, request)
	}
	if reader.idOrHash != "trace" {
		t.Fatalf("SafeReadMultisigRequests called with %q, want trace", reader.idOrHash)
	}
}

func TestValidateSafeReadConsistencyAllowsConcurrentSignature(t *testing.T) {
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersion,
		Asset:   mixinnet.NewHash([]byte("asset")),
		Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	first := &mixin.SafeMultisigRequest{
		RequestID:        "trace",
		AssetID:          "asset",
		KernelAssetID:    tx.Asset,
		Amount:           decimal.NewFromInt(1),
		SendersHash:      "senders",
		SendersThreshold: 2,
		Senders:          []string{"a", "b"},
		RawTransaction:   raw,
	}
	later := *first
	later.Signers = []string{"a"}
	signed := *tx
	signed.Signatures = []map[uint16]*mixinnet.Signature{{0: testSignature(1)}}
	later.RawTransaction, err = signed.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSafeReadConsistency(first, &later); err != nil {
		t.Fatalf("concurrent signature should preserve request identity: %v", err)
	}
}

type fakeSafeMultisigReader struct {
	request  *mixin.SafeMultisigRequest
	idOrHash string
}

func (f *fakeSafeMultisigReader) SafeReadMultisigRequests(_ context.Context, idOrHash string) (*mixin.SafeMultisigRequest, error) {
	f.idOrHash = idOrHash
	return f.request, nil
}

func TestValidateSafeRequestTraceRejectsMisdirectedResponse(t *testing.T) {
	request := &mixin.SafeMultisigRequest{RequestID: "other", TransactionHash: "hash"}
	if err := validateSafeRequestTrace(request, "trace"); err == nil {
		t.Fatal("expected a response for another request to be rejected")
	}
	request.TransactionHash = "trace"
	if err := validateSafeRequestTrace(request, "trace"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSafeRequestRawBindsConfirmedPayload(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	inputHash := mixinnet.NewHash([]byte("input"))
	confirmed := &mixinnet.Transaction{
		Version: mixinnet.TxVersion,
		Asset:   asset,
		Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
		Extra:   []byte("confirmed"),
	}
	confirmedRaw, err := confirmed.Dump()
	if err != nil {
		t.Fatal(err)
	}

	signed := *confirmed
	signed.Signatures = []map[uint16]*mixinnet.Signature{{}}
	signedRaw, err := signed.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSafeRequestRaw(signedRaw, confirmedRaw); err != nil {
		t.Fatalf("signed form of the confirmed payload must match: %v", err)
	}

	conflict := *confirmed
	conflict.Extra = []byte("different")
	conflictRaw, err := conflict.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSafeRequestRaw(conflictRaw, confirmedRaw); err == nil {
		t.Fatal("expected raw payload mismatch")
	}
}

func TestSafeTransferRegistersSignatureCancellation(t *testing.T) {
	cmd := NewCmdTransfer()
	cancel, _, err := cmd.Find([]string{"cancel"})
	if err != nil {
		t.Fatal(err)
	}
	if cancel == nil || cancel.Name() != "cancel" {
		t.Fatal("safe transfer cancel command is not registered")
	}
}

type fakeSafeMultisigSigner struct {
	response *mixin.SafeMultisigRequest
	raw      string
}

func (f *fakeSafeMultisigSigner) SafeSignMultisigRequest(_ context.Context, input *mixin.SafeTransactionRequestInput) (*mixin.SafeMultisigRequest, error) {
	f.raw = input.RawTransaction
	response := *f.response
	response.RawTransaction = input.RawTransaction
	return &response, nil
}

type fakeSafeUtxoLister struct {
	pages   map[uint64][]*mixin.SafeUtxo
	offsets []uint64
}

func (f *fakeSafeUtxoLister) SafeListUtxos(_ context.Context, opt mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error) {
	f.offsets = append(f.offsets, opt.Offset)
	return f.pages[opt.Offset], nil
}

func testSignature(seed byte) *mixinnet.Signature {
	var signature mixinnet.Signature
	signature[0] = seed
	return &signature
}

func safeTransactionFixture(t *testing.T) (*mixinnet.Transaction, string, *mixin.SafeMultisigRequest, []*mixin.SafeTransactionReceiver) {
	t.Helper()
	asset := mixinnet.NewHash([]byte("asset"))
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersion,
		Asset:   asset,
		Inputs:  []*mixinnet.Input{{Hash: &inputHash}},
		Outputs: []*mixinnet.Output{
			{
				Type:   mixinnet.OutputTypeScript,
				Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(2)),
				Script: mixinnet.NewThresholdScript(1),
			},
			{
				Type:   mixinnet.OutputTypeScript,
				Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(3)),
				Script: mixinnet.NewThresholdScript(1),
			},
		},
		Extra: []byte("memo"),
	}
	for _, output := range tx.Outputs {
		output.Mask = mixinnet.GenerateKey(rand.Reader).Public()
		output.Keys = []mixinnet.Key{mixinnet.GenerateKey(rand.Reader).Public()}
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	request := &mixin.SafeMultisigRequest{
		RequestID:        "trace",
		KernelAssetID:    asset,
		AssetID:          "asset",
		Amount:           decimal.NewFromInt(2),
		Extra:            "memo",
		Senders:          []string{"sender"},
		SendersThreshold: 1,
		RawTransaction:   raw,
	}
	receivers := []*mixin.SafeTransactionReceiver{
		{Members: []string{"receiver"}, Threshold: 1},
		{Members: []string{"sender"}, Threshold: 1},
	}
	return tx, raw, request, receivers
}
