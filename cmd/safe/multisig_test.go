package safe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
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
	return f.response, nil
}
