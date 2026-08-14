package transfer

import (
	"context"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
)

func TestSelectLegacyMultisigOutputsAcceptsExactBalance(t *testing.T) {
	outputs := []*mixin.MultisigUTXO{
		{Amount: decimal.RequireFromString("0.5"), State: mixin.UTXOStateUnspent},
		{Amount: decimal.RequireFromString("1.5"), State: mixin.UTXOStateUnspent},
	}
	selected, err := selectLegacyMultisigOutputs(outputs, decimal.NewFromInt(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("selected %d outputs, want 2", len(selected))
	}
}

func TestValidateLegacyMultisigTransfer(t *testing.T) {
	input := mixin.TransferInput{
		AssetID:    "asset",
		Amount:     decimal.NewFromInt(1),
		TraceID:    "trace",
		OpponentID: "00000000-0000-0000-0000-000000000003",
	}
	senders := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	if err := validateLegacyMultisigTransfer(input, senders, 2); err != nil {
		t.Fatal(err)
	}

	input.TraceID = ""
	if err := validateLegacyMultisigTransfer(input, senders, 2); err == nil {
		t.Fatal("expected missing trace error")
	}
	input.TraceID = "trace"
	input.OpponentMultisig.Receivers = []string{"00000000-0000-0000-0000-000000000004"}
	input.OpponentMultisig.Threshold = 1
	if err := validateLegacyMultisigTransfer(input, senders, 2); err == nil {
		t.Fatal("expected mutually exclusive destination error")
	}

	input.OpponentMultisig.Receivers = nil
	input.OpponentMultisig.Threshold = 0
	input.Amount = decimal.RequireFromString("0.000000001")
	if err := validateLegacyMultisigTransfer(input, senders, 2); err == nil {
		t.Fatal("expected precision error")
	}
}

func TestValidateLegacySourceOutputsRejectsUnsafeIndex(t *testing.T) {
	senders := []string{"00000000-0000-0000-0000-000000000001"}
	outputs := []*mixin.MultisigUTXO{{
		UTXOID:      "output",
		AssetID:     "asset",
		Amount:      decimal.NewFromInt(1),
		Members:     senders,
		Threshold:   1,
		OutputIndex: 256,
	}}
	if err := validateLegacySourceOutputs(outputs, senders, 1, "asset"); err == nil {
		t.Fatal("expected unsafe output index error")
	}
}

func TestValidateLegacyMultisigRequestIgnoresMemberOrder(t *testing.T) {
	input := mixin.TransferInput{
		AssetID:    "asset",
		Amount:     decimal.NewFromInt(1),
		Memo:       "memo",
		OpponentID: "receiver",
	}
	request := &mixin.MultisigRequest{
		AssetID:   "asset",
		Amount:    decimal.NewFromInt(1),
		Memo:      "memo",
		Threshold: 2,
		Senders:   []string{"b", "a"},
		Receivers: []string{"receiver"},
	}
	if err := validateLegacyMultisigRequest(request, input, []string{"a", "b"}, 2); err != nil {
		t.Fatal(err)
	}

	request.Amount = decimal.NewFromInt(2)
	if err := validateLegacyMultisigRequest(request, input, []string{"a", "b"}, 2); err == nil {
		t.Fatal("expected amount mismatch")
	}
}

func TestLegacyRawMatchesTransferInputs(t *testing.T) {
	assetID := "asset"
	hash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersionLegacy,
		Asset:   mixinnet.NewHash([]byte(assetID)),
		Inputs: []*mixinnet.Input{{
			Hash:  &hash,
			Index: 0,
		}},
		Outputs: []*mixinnet.Output{{
			Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1)),
			Script: mixinnet.NewThresholdScript(1),
		}},
		Extra: []byte("memo"),
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	outputs := []*mixin.MultisigUTXO{{
		AssetID:         assetID,
		TransactionHash: hash,
		OutputIndex:     0,
		Amount:          decimal.NewFromInt(1),
		Members:         []string{"00000000-0000-0000-0000-000000000001"},
		Threshold:       1,
	}}
	receiver := mixin.RequireNewMixAddress([]string{"00000000-0000-0000-0000-000000000002"}, 1)
	input := mixin.TransferInput{AssetID: assetID, Amount: decimal.NewFromInt(1), TraceID: "trace", Memo: "memo"}
	senders := []string{"00000000-0000-0000-0000-000000000001"}
	matched, err := legacyRawMatchesTransfer(context.Background(), fakeLegacyTransactionMaker{tx: tx}, raw, outputs, input, senders, 1, receiver)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatal("expected raw transaction to match")
	}

	outputs[0].OutputIndex = 1
	matched, err = legacyRawMatchesTransfer(context.Background(), fakeLegacyTransactionMaker{tx: tx}, raw, outputs, input, senders, 1, receiver)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("transaction with different inputs must not match")
	}
}

func TestLegacyTransferRegistersCancellationCommands(t *testing.T) {
	cmd := NewCmdTransfer()
	for _, name := range []string{"cancel", "cancel-request"} {
		child, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if child == nil || child.Name() != name {
			t.Fatalf("transfer %s command is not registered", name)
		}
	}
}

type fakeLegacyTransactionMaker struct {
	tx *mixinnet.Transaction
}

func (f fakeLegacyTransactionMaker) MakeTransaction(context.Context, *mixin.TransactionBuilder, []*mixin.TransactionOutput) (*mixinnet.Transaction, error) {
	return f.tx, nil
}
