package transfer

import (
	"bytes"
	"context"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
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

func TestSelectLegacyMultisigOutputsRejectsNilOutput(t *testing.T) {
	if _, err := selectLegacyMultisigOutputs([]*mixin.MultisigUTXO{nil}, decimal.NewFromInt(1)); err == nil {
		t.Fatal("expected nil output error")
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

func TestValidateLegacySourceOutputsRejectsInconsistentMemberOrder(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	outputs := []*mixin.MultisigUTXO{
		{
			UTXOID:    "first",
			AssetID:   "asset",
			Amount:    decimal.NewFromInt(1),
			Members:   []string{members[0], members[1]},
			Threshold: 2,
		},
		{
			UTXOID:    "second",
			AssetID:   "asset",
			Amount:    decimal.NewFromInt(1),
			Members:   []string{members[1], members[0]},
			Threshold: 2,
		},
	}
	if err := validateLegacySourceOutputs(outputs, members, 2, "asset"); err == nil {
		t.Fatal("expected inconsistent source address error")
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
		RequestID: "request",
		AssetID:   "asset",
		Amount:    decimal.NewFromInt(1),
		Memo:      "memo",
		Threshold: 2,
		Senders:   []string{"b", "a"},
		Receivers: []string{"receiver"},
		Action:    mixin.MultisigActionSign,
	}
	if err := validateLegacyMultisigRequest(request, input, []string{"a", "b"}, 2); err != nil {
		t.Fatal(err)
	}

	request.Amount = decimal.NewFromInt(2)
	if err := validateLegacyMultisigRequest(request, input, []string{"a", "b"}, 2); err == nil {
		t.Fatal("expected amount mismatch")
	}
}

func TestValidateLegacyRequestRawBindsConfirmedPayload(t *testing.T) {
	asset := mixinnet.NewHash([]byte("asset"))
	inputHash := mixinnet.NewHash([]byte("input"))
	confirmed := &mixinnet.Transaction{
		Version: mixinnet.TxVersionLegacy,
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
	if err := validateLegacyRequestRaw(signedRaw, confirmedRaw); err != nil {
		t.Fatalf("signed form of the confirmed payload must match: %v", err)
	}

	conflict := *confirmed
	conflict.Extra = []byte("different")
	conflictRaw, err := conflict.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLegacyRequestRaw(conflictRaw, confirmedRaw); err == nil {
		t.Fatal("expected raw payload mismatch")
	}
}

func TestProcessLegacyMultisigRequestJoinsAndBroadcasts(t *testing.T) {
	assetID := "asset"
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersionLegacy,
		Asset:   mixinnet.NewHash([]byte(assetID)),
		Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	members := []string{"a", "b"}
	request := func(signers []string) *mixin.MultisigRequest {
		return &mixin.MultisigRequest{
			RequestID:      "request",
			AssetID:        assetID,
			Amount:         decimal.NewFromInt(1),
			Threshold:      2,
			Senders:        members,
			Receivers:      []string{"receiver"},
			Signers:        signers,
			Action:         mixin.MultisigActionSign,
			RawTransaction: raw,
		}
	}
	client := &fakeLegacyRequestClient{
		created: request([]string{"a"}),
		signed:  request([]string{"a", "b"}),
	}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	broadcasts := 0
	err = processLegacyMultisigRequest(
		cmd,
		client,
		"b",
		raw,
		mixin.TransferInput{AssetID: assetID, Amount: decimal.NewFromInt(1), OpponentID: "receiver"},
		members,
		2,
		func() (string, error) { return "pin", nil },
		func(_ context.Context, candidate string) (*mixinnet.Transaction, error) {
			broadcasts++
			if candidate != raw {
				t.Fatalf("broadcast raw mismatch")
			}
			hash := mixinnet.NewHash([]byte("result"))
			return &mixinnet.Transaction{Hash: &hash}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if client.signCalls != 1 || broadcasts != 1 {
		t.Fatalf("sign calls = %d, broadcasts = %d; want 1 each", client.signCalls, broadcasts)
	}
}

func TestProcessLegacyMultisigRequestRejectsChangedSignedPayload(t *testing.T) {
	assetID := "asset"
	inputHash := mixinnet.NewHash([]byte("input"))
	transaction := func(extra string) string {
		tx := &mixinnet.Transaction{
			Version: mixinnet.TxVersionLegacy,
			Asset:   mixinnet.NewHash([]byte(assetID)),
			Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
			Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
			Extra:   []byte(extra),
		}
		raw, err := tx.Dump()
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	confirmedRaw := transaction("confirmed")
	changedRaw := transaction("changed")
	members := []string{"a", "b"}
	request := func(signers []string, raw string) *mixin.MultisigRequest {
		return &mixin.MultisigRequest{
			RequestID:      "request",
			AssetID:        assetID,
			Amount:         decimal.NewFromInt(1),
			Threshold:      2,
			Senders:        members,
			Receivers:      []string{"receiver"},
			Signers:        signers,
			Action:         mixin.MultisigActionSign,
			RawTransaction: raw,
		}
	}
	client := &fakeLegacyRequestClient{
		created: request([]string{"a"}, confirmedRaw),
		signed:  request([]string{"a", "b"}, changedRaw),
	}
	cmd := &cobra.Command{}
	broadcasts := 0
	err := processLegacyMultisigRequest(
		cmd,
		client,
		"b",
		confirmedRaw,
		mixin.TransferInput{AssetID: assetID, Amount: decimal.NewFromInt(1), OpponentID: "receiver"},
		members,
		2,
		func() (string, error) { return "pin", nil },
		func(context.Context, string) (*mixinnet.Transaction, error) {
			broadcasts++
			return nil, nil
		},
	)
	if err == nil {
		t.Fatal("expected changed signed payload to be rejected")
	}
	if broadcasts != 0 {
		t.Fatalf("unexpected broadcasts: %d", broadcasts)
	}
}

func TestProcessLegacyMultisigRequestRejectsNonMonotonicSigners(t *testing.T) {
	assetID := "asset"
	inputHash := mixinnet.NewHash([]byte("input"))
	tx := &mixinnet.Transaction{
		Version: mixinnet.TxVersionLegacy,
		Asset:   mixinnet.NewHash([]byte(assetID)),
		Inputs:  []*mixinnet.Input{{Hash: &inputHash, Index: 0}},
		Outputs: []*mixinnet.Output{{Amount: mixinnet.IntegerFromDecimal(decimal.NewFromInt(1))}},
	}
	raw, err := tx.Dump()
	if err != nil {
		t.Fatal(err)
	}
	members := []string{"a", "b", "c"}
	request := func(signers []string) *mixin.MultisigRequest {
		return &mixin.MultisigRequest{
			RequestID:      "request",
			AssetID:        assetID,
			Amount:         decimal.NewFromInt(1),
			Threshold:      2,
			Senders:        members,
			Receivers:      []string{"receiver"},
			Signers:        signers,
			Action:         mixin.MultisigActionSign,
			RawTransaction: raw,
		}
	}
	client := &fakeLegacyRequestClient{
		created: request([]string{"a"}),
		signed:  request([]string{"b"}),
	}
	broadcasts := 0
	err = processLegacyMultisigRequest(
		&cobra.Command{},
		client,
		"b",
		raw,
		mixin.TransferInput{AssetID: assetID, Amount: decimal.NewFromInt(1), OpponentID: "receiver"},
		members,
		2,
		func() (string, error) { return "pin", nil },
		func(context.Context, string) (*mixinnet.Transaction, error) {
			broadcasts++
			return nil, nil
		},
	)
	if err == nil {
		t.Fatal("expected non-monotonic signer response to be rejected")
	}
	if broadcasts != 0 {
		t.Fatalf("unexpected broadcasts: %d", broadcasts)
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

type fakeLegacyRequestClient struct {
	created   *mixin.MultisigRequest
	signed    *mixin.MultisigRequest
	signCalls int
}

func (f *fakeLegacyRequestClient) CreateMultisig(context.Context, string, string) (*mixin.MultisigRequest, error) {
	return f.created, nil
}

func (f *fakeLegacyRequestClient) SignMultisig(context.Context, string, string) (*mixin.MultisigRequest, error) {
	f.signCalls++
	return f.signed, nil
}

func (f fakeLegacyTransactionMaker) MakeTransaction(context.Context, *mixin.TransactionBuilder, []*mixin.TransactionOutput) (*mixinnet.Transaction, error) {
	return f.tx, nil
}
