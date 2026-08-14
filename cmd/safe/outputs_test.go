package safe

import (
	"context"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
)

type fakeSafeUtxoLister struct {
	calls   []mixin.SafeListUtxoOption
	outputs [][]*mixin.SafeUtxo
}

func (f *fakeSafeUtxoLister) SafeListUtxos(_ context.Context, opt mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error) {
	f.calls = append(f.calls, opt)
	return f.outputs[len(f.calls)-1], nil
}

func TestListUnspentOutputsForSafeMultisig(t *testing.T) {
	firstPage := make([]*mixin.SafeUtxo, 256)
	for i := range firstPage {
		firstPage[i] = &mixin.SafeUtxo{
			AssetID:  "asset-a",
			Amount:   decimal.NewFromInt(1),
			Sequence: uint64(i + 10),
		}
	}
	secondPage := []*mixin.SafeUtxo{{
		AssetID:  "asset-b",
		Amount:   decimal.NewFromInt(2),
		Sequence: 266,
	}}
	lister := &fakeSafeUtxoLister{outputs: [][]*mixin.SafeUtxo{firstPage, secondPage}}
	members := []string{"member-a", "member-b"}

	outputs, err := listUnspentOutputs(context.Background(), lister, members, 2)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(outputs["asset-a"]); got != 256 {
		t.Fatalf("asset-a output count = %d, want 256", got)
	}
	if got := len(outputs["asset-b"]); got != 1 {
		t.Fatalf("asset-b output count = %d, want 1", got)
	}
	if got := len(lister.calls); got != 2 {
		t.Fatalf("call count = %d, want 2", got)
	}
	for _, call := range lister.calls {
		if call.Order != "ASC" || call.State != mixin.SafeUtxoStateUnspent || call.Limit != 256 {
			t.Fatalf("unexpected list option: %+v", call)
		}
		if call.Threshold != 2 || len(call.Members) != 2 {
			t.Fatalf("multisig group not forwarded: %+v", call)
		}
	}
	if got := lister.calls[1].Offset; got != 266 {
		t.Fatalf("second page offset = %d, want 266", got)
	}
}

func TestValidateMultisigGroup(t *testing.T) {
	tests := []struct {
		name      string
		members   []string
		threshold uint8
		wantErr   bool
	}{
		{name: "own safe", wantErr: false},
		{name: "safe multisig", members: []string{"a", "b"}, threshold: 2, wantErr: false},
		{name: "missing members", threshold: 1, wantErr: true},
		{name: "missing threshold", members: []string{"a"}, wantErr: true},
		{name: "threshold too large", members: []string{"a"}, threshold: 2, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMultisigGroup(tt.members, tt.threshold)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateMultisigGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
