package safe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
)

type fakeSafeUtxoLister struct {
	calls   []mixin.SafeListUtxoOption
	outputs [][]*mixin.SafeUtxo
}

type stalledSafeUtxoLister struct {
	calls int
	page  []*mixin.SafeUtxo
}

func (f *stalledSafeUtxoLister) SafeListUtxos(context.Context, mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error) {
	f.calls++
	if f.calls > 2 {
		return nil, errors.New("unexpected third request")
	}
	return f.page, nil
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

func TestListUnspentOutputsForMixAddress(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	address, err := mixin.NewMixAddress(members, 2)
	if err != nil {
		t.Fatal(err)
	}
	lister := &fakeSafeUtxoLister{outputs: [][]*mixin.SafeUtxo{nil}}

	if _, err := listUnspentOutputs(context.Background(), lister, []string{address.String()}, 0); err != nil {
		t.Fatal(err)
	}
	if len(lister.calls) != 1 {
		t.Fatalf("call count = %d, want 1", len(lister.calls))
	}
	call := lister.calls[0]
	if call.Threshold != 2 {
		t.Fatalf("threshold = %d, want 2", call.Threshold)
	}
	if got := strings.Join(call.Members, ","); got != strings.Join(members, ",") {
		t.Fatalf("members = %q, want %q", got, strings.Join(members, ","))
	}
}

func TestListUnspentOutputsRejectsStalledCursor(t *testing.T) {
	page := make([]*mixin.SafeUtxo, 256)
	for i := range page {
		page[i] = &mixin.SafeUtxo{AssetID: "asset-a", Sequence: uint64(i + 1)}
	}
	lister := &stalledSafeUtxoLister{page: page}

	_, err := listUnspentOutputs(context.Background(), lister, []string{"member-a"}, 1)
	if err == nil || !strings.Contains(err.Error(), "pagination stalled") {
		t.Fatalf("error = %v, want pagination stalled", err)
	}
	if lister.calls != 2 {
		t.Fatalf("call count = %d, want 2", lister.calls)
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

func TestNewCmdAssetsAdvertisesMixAddress(t *testing.T) {
	flag := NewCmdAssets().Flags().Lookup("receivers")
	if flag == nil || !strings.Contains(flag.Usage, "MIX address") {
		t.Fatalf("receivers flag = %#v, want MIX address support", flag)
	}
}
