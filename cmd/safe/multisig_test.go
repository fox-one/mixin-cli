package safe

import (
	"encoding/json"
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
