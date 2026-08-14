package list

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
)

type fakeLegacyMultisigOutputLister struct {
	calls        []mixin.ListMultisigOutputsOption
	outputs      [][]*mixin.MultisigUTXO
	assets       []*mixin.SafeAsset
	asset        *mixin.SafeAsset
	assetErr     error
	readAssetErr error
}

func (f *fakeLegacyMultisigOutputLister) ListMultisigOutputs(_ context.Context, opt mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error) {
	f.calls = append(f.calls, opt)
	return f.outputs[len(f.calls)-1], nil
}

func (f *fakeLegacyMultisigOutputLister) SafeFetchAssets(context.Context, []string) ([]*mixin.SafeAsset, error) {
	return f.assets, f.assetErr
}

func (f *fakeLegacyMultisigOutputLister) SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error) {
	return f.asset, f.readAssetErr
}

func TestReadLegacyMultisigBalances(t *testing.T) {
	firstCreatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("utxo-%d", i),
			AssetID:   "asset-a",
			Amount:    decimal.NewFromInt(1),
			CreatedAt: firstCreatedAt.Add(time.Duration(i) * time.Second),
		}
	}
	secondPage := []*mixin.MultisigUTXO{
		firstPage[len(firstPage)-1],
		{
			UTXOID:    "utxo-500",
			AssetID:   "asset-b",
			Amount:    decimal.NewFromInt(2),
			CreatedAt: firstCreatedAt.Add(500 * time.Second),
		},
	}
	lister := &fakeLegacyMultisigOutputLister{outputs: [][]*mixin.MultisigUTXO{firstPage, secondPage}}
	receivers := []string{"member-a", "member-b"}

	balances, err := readLegacyMultisigBalances(context.Background(), lister, receivers, 2)
	if err != nil {
		t.Fatal(err)
	}

	if got := balances["asset-a"]; !got.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("asset-a balance = %s, want 500", got)
	}
	if got := balances["asset-b"]; !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("asset-b balance = %s, want 2", got)
	}
	if got := len(lister.calls); got != 2 {
		t.Fatalf("call count = %d, want 2", got)
	}
	for _, call := range lister.calls {
		if !call.OrderByCreated || call.State != mixin.UTXOStateUnspent || call.Limit != 500 {
			t.Fatalf("unexpected list option: %+v", call)
		}
		if call.Threshold != 2 || len(call.Members) != 2 {
			t.Fatalf("multisig group not forwarded: %+v", call)
		}
	}
	wantOffset := firstPage[len(firstPage)-1].CreatedAt
	if got := lister.calls[1].Offset; !got.Equal(wantOffset) {
		t.Fatalf("second page offset = %s, want %s", got, wantOffset)
	}
}

func TestReadLegacyMultisigAssetsUsesSafeMetadata(t *testing.T) {
	lister := &fakeLegacyMultisigOutputLister{
		outputs: [][]*mixin.MultisigUTXO{{{
			AssetID: "asset-a",
			Amount:  decimal.RequireFromString("12.34"),
		}}},
		assets: []*mixin.SafeAsset{{
			AssetID:  "asset-a",
			Symbol:   "BTC",
			Name:     "Bitcoin",
			PriceUSD: decimal.RequireFromString("100"),
		}},
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{"member-a"}
	input.OpponentMultisig.Threshold = 1

	assets, err := readLegacyMultisigAssets(context.Background(), lister, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1", len(assets))
	}
	asset := assets[0]
	if asset.AssetID != "asset-a" || asset.Symbol != "BTC" || asset.Name != "Bitcoin" {
		t.Fatalf("unexpected asset metadata: %+v", asset)
	}
	if !asset.Balance.Equal(decimal.RequireFromString("12.34")) {
		t.Fatalf("balance = %s, want 12.34", asset.Balance)
	}
}

func TestReadLegacyMultisigBalancesRejectsStalledCursor(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	secondPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("first-%d", i),
			AssetID:   "asset-a",
			Amount:    decimal.NewFromInt(1),
			CreatedAt: createdAt,
		}
		secondPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("second-%d", i),
			AssetID:   "asset-a",
			Amount:    decimal.NewFromInt(1),
			CreatedAt: createdAt,
		}
	}
	lister := &fakeLegacyMultisigOutputLister{outputs: [][]*mixin.MultisigUTXO{firstPage, secondPage}}

	_, err := readLegacyMultisigBalances(context.Background(), lister, []string{"member-a"}, 1)
	if err == nil {
		t.Fatal("expected stalled cursor error")
	}
}

func TestReadLegacyMultisigAssetsReturnsMetadataError(t *testing.T) {
	lister := &fakeLegacyMultisigOutputLister{
		outputs: [][]*mixin.MultisigUTXO{{{
			UTXOID:  "utxo-1",
			AssetID: "asset-a",
			Amount:  decimal.NewFromInt(1),
		}}},
		assetErr: errors.New("metadata unavailable"),
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{"member-a"}
	input.OpponentMultisig.Threshold = 1

	_, err := readLegacyMultisigAssets(context.Background(), lister, input)
	if err == nil {
		t.Fatal("expected metadata error")
	}
}

func TestReadLegacyMultisigAssetsFetchesMissingMetadata(t *testing.T) {
	lister := &fakeLegacyMultisigOutputLister{
		outputs: [][]*mixin.MultisigUTXO{{{
			UTXOID:  "utxo-1",
			AssetID: "asset-a",
			Amount:  decimal.NewFromInt(1),
		}}},
		asset: &mixin.SafeAsset{AssetID: "asset-a", Symbol: "BTC"},
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{"member-a"}
	input.OpponentMultisig.Threshold = 1

	assets, err := readLegacyMultisigAssets(context.Background(), lister, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Symbol != "BTC" {
		t.Fatalf("unexpected assets: %+v", assets)
	}
}

func TestValidateLegacyMultisigGroup(t *testing.T) {
	tests := []struct {
		name      string
		receivers []string
		threshold uint8
		wantErr   bool
	}{
		{name: "valid", receivers: []string{"a", "b"}, threshold: 2},
		{name: "missing receivers", threshold: 1, wantErr: true},
		{name: "missing threshold", receivers: []string{"a"}, wantErr: true},
		{name: "threshold too large", receivers: []string{"a"}, threshold: 2, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLegacyMultisigGroup(tt.receivers, tt.threshold)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLegacyMultisigGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
