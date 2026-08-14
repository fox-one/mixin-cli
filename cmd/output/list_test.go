package output

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fox-one/mixin-sdk-go/v2"
)

func TestValidateListOptions(t *testing.T) {
	tests := []struct {
		name    string
		opt     listOptions
		wantErr bool
	}{
		{name: "own safe", opt: listOptions{limit: 100, order: "DESC"}},
		{name: "safe multisig", opt: listOptions{receivers: []string{"a", "b"}, threshold: 2, limit: 100, order: "ASC"}},
		{name: "legacy multisig", opt: listOptions{legacy: true, receivers: []string{"a"}, threshold: 1, state: "unspent", limit: 100, order: "DESC"}},
		{name: "missing receivers", opt: listOptions{threshold: 1, limit: 100, order: "DESC"}, wantErr: true},
		{name: "missing threshold", opt: listOptions{receivers: []string{"a"}, limit: 100, order: "DESC"}, wantErr: true},
		{name: "threshold too large", opt: listOptions{receivers: []string{"a"}, threshold: 2, limit: 100, order: "DESC"}, wantErr: true},
		{name: "invalid state", opt: listOptions{state: "unknown", limit: 100, order: "DESC"}, wantErr: true},
		{name: "invalid limit", opt: listOptions{limit: 501, order: "DESC"}, wantErr: true},
		{name: "invalid order", opt: listOptions{limit: 100, order: "random"}, wantErr: true},
		{name: "legacy ascending", opt: listOptions{legacy: true, limit: 100, order: "ASC"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListOptions(tt.opt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateListOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type fakeSafeOutputClient struct {
	calls []mixin.SafeListUtxoOption
	pages [][]*mixin.SafeUtxo
}

func (f *fakeSafeOutputClient) SafeListUtxos(_ context.Context, opt mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error) {
	f.calls = append(f.calls, opt)
	return f.pages[len(f.calls)-1], nil
}

func (f *fakeSafeOutputClient) SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error) {
	return nil, nil
}

func TestListSafeOutputsDescending(t *testing.T) {
	firstPage := make([]*mixin.SafeUtxo, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.SafeUtxo{Sequence: uint64(i + 1)}
	}
	secondPage := []*mixin.SafeUtxo{{Sequence: 501}, {Sequence: 502}}
	client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{firstPage, secondPage}}

	outputs, err := listSafeOutputs(context.Background(), client, listOptions{limit: 3, order: "DESC"})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{502, 501, 500}
	if len(outputs) != len(want) {
		t.Fatalf("output count = %d, want %d", len(outputs), len(want))
	}
	for i, output := range outputs {
		if output.Sequence != want[i] {
			t.Fatalf("output[%d].Sequence = %d, want %d", i, output.Sequence, want[i])
		}
	}
	for _, call := range client.calls {
		if call.Order != "ASC" {
			t.Fatalf("API order = %q, want ASC", call.Order)
		}
	}
}

func TestListSafeOutputsDescendingExcludesOffset(t *testing.T) {
	client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{{
		{Sequence: 1},
		{Sequence: 2},
		{Sequence: 3},
		{Sequence: 4},
		{Sequence: 5},
	}}}

	outputs, err := listSafeOutputs(context.Background(), client, listOptions{
		offset: "4",
		limit:  2,
		order:  "DESC",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{3, 2}
	if len(outputs) != len(want) {
		t.Fatalf("output count = %d, want %d", len(outputs), len(want))
	}
	for i, output := range outputs {
		if output.Sequence != want[i] {
			t.Fatalf("output[%d].Sequence = %d, want %d", i, output.Sequence, want[i])
		}
	}
}

func TestListSafeOutputsDescendingRejectsStalledCursor(t *testing.T) {
	page := make([]*mixin.SafeUtxo, 500)
	for i := range page {
		page[i] = &mixin.SafeUtxo{Sequence: uint64(i + 1)}
	}
	client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{page, page}}

	_, err := listSafeOutputs(context.Background(), client, listOptions{
		limit: 1,
		order: "DESC",
	})
	if err == nil || !strings.Contains(err.Error(), "pagination stalled") {
		t.Fatalf("error = %v, want pagination stalled", err)
	}
}

type fakeLegacyOutputLister struct {
	calls    []mixin.ListMultisigOutputsOption
	pages    [][]*mixin.MultisigUTXO
	asset    *mixin.SafeAsset
	assetErr error
}

func (f *fakeLegacyOutputLister) ListMultisigOutputs(_ context.Context, opt mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error) {
	f.calls = append(f.calls, opt)
	return f.pages[len(f.calls)-1], nil
}

func (f *fakeLegacyOutputLister) SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error) {
	return f.asset, f.assetErr
}

func TestListLegacyOutputsDescending(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("utxo-%d", i),
			AssetID:   "asset-a",
			CreatedAt: start.Add(time.Duration(i) * time.Second),
		}
	}
	secondPage := []*mixin.MultisigUTXO{
		firstPage[len(firstPage)-1],
		{UTXOID: "501", AssetID: "asset-a", CreatedAt: start.Add(500 * time.Second)},
		{UTXOID: "502", AssetID: "asset-b", CreatedAt: start.Add(501 * time.Second)},
	}
	client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{firstPage, secondPage}}

	outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
		legacy: true,
		asset:  "asset-a",
		limit:  3,
		order:  "DESC",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{
		start.Add(500 * time.Second),
		start.Add(499 * time.Second),
		start.Add(498 * time.Second),
	}
	if len(outputs) != len(want) {
		t.Fatalf("output count = %d, want %d", len(outputs), len(want))
	}
	for i, output := range outputs {
		if !output.CreatedAt.Equal(want[i]) {
			t.Fatalf("output[%d].CreatedAt = %s, want %s", i, output.CreatedAt, want[i])
		}
	}
}

func TestListLegacyOutputsAscendingFillsAssetLimit(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("utxo-%d", i),
			AssetID:   "other-asset",
			CreatedAt: start.Add(time.Duration(i) * time.Second),
		}
	}
	secondPage := []*mixin.MultisigUTXO{
		firstPage[len(firstPage)-1],
		{UTXOID: "utxo-500", AssetID: "asset-a", CreatedAt: start.Add(500 * time.Second)},
		{UTXOID: "utxo-501", AssetID: "asset-a", CreatedAt: start.Add(501 * time.Second)},
	}
	client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{firstPage, secondPage}}

	outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
		legacy: true,
		asset:  "asset-a",
		limit:  2,
		order:  "ASC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 2 {
		t.Fatalf("output count = %d, want 2", len(outputs))
	}
	if !outputs[0].CreatedAt.Before(outputs[1].CreatedAt) {
		t.Fatalf("outputs are not ascending: %s >= %s", outputs[0].CreatedAt, outputs[1].CreatedAt)
	}
}

func TestListLegacyOutputsAscendingExcludesOffset(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	page := []*mixin.MultisigUTXO{
		{UTXOID: "boundary", AssetID: "asset-a", CreatedAt: createdAt},
		{UTXOID: "next", AssetID: "asset-a", CreatedAt: createdAt.Add(time.Second)},
	}

	for _, assetID := range []string{"", "asset-a"} {
		t.Run("asset="+assetID, func(t *testing.T) {
			client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{page}}
			outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
				legacy: true,
				asset:  assetID,
				offset: createdAt.Format(time.RFC3339Nano),
				limit:  1,
				order:  "ASC",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(outputs) != 1 || outputs[0].UTXOID != "next" {
				t.Fatalf("outputs = %+v, want only next output", outputs)
			}
		})
	}
}

func TestListLegacyOutputsResolvesKernelAssetID(t *testing.T) {
	kernelAssetID := strings.Repeat("a", 64)
	client := &fakeLegacyOutputLister{
		asset: &mixin.SafeAsset{AssetID: "asset-a", KernelAssetID: kernelAssetID},
		pages: [][]*mixin.MultisigUTXO{{{
			UTXOID:  "utxo-1",
			AssetID: "asset-a",
		}}},
	}

	outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
		legacy: true,
		asset:  kernelAssetID,
		limit:  1,
		order:  "ASC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 || outputs[0].AssetID != "asset-a" {
		t.Fatalf("unexpected outputs: %+v", outputs)
	}
}

func TestListLegacyOutputsReturnsAssetResolutionError(t *testing.T) {
	client := &fakeLegacyOutputLister{assetErr: errors.New("metadata unavailable")}
	_, err := listLegacyOutputs(context.Background(), client, listOptions{
		legacy: true,
		asset:  strings.Repeat("a", 64),
		limit:  1,
		order:  "ASC",
	})
	if err == nil {
		t.Fatal("expected asset resolution error")
	}
}

func TestListLegacyOutputsRejectsStalledCursor(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	secondPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{UTXOID: fmt.Sprintf("first-%d", i), AssetID: "asset-a", CreatedAt: createdAt}
		secondPage[i] = &mixin.MultisigUTXO{UTXOID: fmt.Sprintf("second-%d", i), AssetID: "asset-a", CreatedAt: createdAt}
	}
	client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{firstPage, secondPage}}

	_, err := listLegacyOutputs(context.Background(), client, listOptions{
		legacy: true,
		asset:  "target-asset",
		limit:  500,
		order:  "ASC",
	})
	if err == nil {
		t.Fatal("expected stalled cursor error")
	}
}

func TestListOutputsReturnEmptyArrays(t *testing.T) {
	safeClient := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{nil}}
	safeOutputs, err := listSafeOutputs(context.Background(), safeClient, listOptions{limit: 1, order: "ASC"})
	if err != nil {
		t.Fatal(err)
	}
	if safeOutputs == nil {
		t.Fatal("safe outputs must be an empty array, not nil")
	}

	legacyClient := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{nil}}
	legacyOutputs, err := listLegacyOutputs(context.Background(), legacyClient, listOptions{legacy: true, limit: 1, order: "ASC"})
	if err != nil {
		t.Fatal(err)
	}
	if legacyOutputs == nil {
		t.Fatal("legacy outputs must be an empty array, not nil")
	}
}

func TestNewCmdListFlags(t *testing.T) {
	cmd := NewCmdList()
	for _, name := range []string{"legacy", "receivers", "threshold", "asset", "state", "offset", "limit", "order"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
	if got := cmd.Flags().Lookup("order").DefValue; got != "ASC" {
		t.Fatalf("default order = %q, want ASC", got)
	}
}
