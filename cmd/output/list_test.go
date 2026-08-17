package output

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
)

func TestValidateListOptions(t *testing.T) {
	tests := []struct {
		name    string
		opt     listOptions
		wantErr bool
	}{
		{name: "own safe", opt: listOptions{limit: 100, order: "DESC"}},
		{name: "safe multisig", opt: listOptions{receivers: []string{"a", "b"}, threshold: 2, limit: 100, order: "ASC"}},
		{name: "multisig", opt: listOptions{receivers: []string{"a"}, threshold: 1, state: "unspent", limit: 100, order: "DESC"}},
		{name: "missing receivers", opt: listOptions{threshold: 1, limit: 100, order: "DESC"}, wantErr: true},
		{name: "missing threshold", opt: listOptions{receivers: []string{"a"}, limit: 100, order: "DESC"}, wantErr: true},
		{name: "threshold too large", opt: listOptions{receivers: []string{"a"}, threshold: 2, limit: 100, order: "DESC"}, wantErr: true},
		{name: "invalid state", opt: listOptions{state: "unknown", limit: 100, order: "DESC"}, wantErr: true},
		{name: "negative limit", opt: listOptions{limit: -1, order: "DESC"}, wantErr: true},
		{name: "limit above API page size", opt: listOptions{limit: 501, order: "DESC"}},
		{name: "invalid order", opt: listOptions{limit: 100, order: "random"}, wantErr: true},
		{name: "ascending", opt: listOptions{limit: 100, order: "ASC"}},
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

func TestNormalizeListOptionsExpandsMixAddress(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	address, err := mixin.NewMixAddress(members, 2)
	if err != nil {
		t.Fatal(err)
	}

	opt := listOptions{receivers: []string{address.String()}}
	if err := normalizeListOptions(&opt); err != nil {
		t.Fatal(err)
	}
	if opt.threshold != 2 {
		t.Fatalf("threshold = %d, want 2", opt.threshold)
	}
	if got := strings.Join(opt.receivers, ","); got != strings.Join(members, ",") {
		t.Fatalf("receivers = %q, want %q", got, strings.Join(members, ","))
	}
}

func TestNormalizeListOptionsExpandsMainnetMixAddress(t *testing.T) {
	members := []string{
		mixinnet.GenerateAddress(rand.Reader, true).String(),
		mixinnet.GenerateAddress(rand.Reader, true).String(),
	}
	address, err := mixin.NewMainnetMixAddress(members, 1)
	if err != nil {
		t.Fatal(err)
	}

	opt := listOptions{receivers: []string{address.String()}}
	if err := normalizeListOptions(&opt); err != nil {
		t.Fatal(err)
	}
	if opt.threshold != 1 {
		t.Fatalf("threshold = %d, want 1", opt.threshold)
	}
	if got := strings.Join(opt.receivers, ","); got != strings.Join(members, ",") {
		t.Fatalf("receivers = %q, want %q", got, strings.Join(members, ","))
	}
}

func TestNormalizeListOptionsKeepsExplicitMembers(t *testing.T) {
	opt := listOptions{receivers: []string{"member-a", "member-b"}, threshold: 2}
	if err := normalizeListOptions(&opt); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(opt.receivers, ","); got != "member-a,member-b" || opt.threshold != 2 {
		t.Fatalf("normalized options = %q/%d", got, opt.threshold)
	}
}

func TestNormalizeListOptionsRejectsInvalidMixAddressInput(t *testing.T) {
	address, err := mixin.NewMixAddress([]string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}, 2)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opt  listOptions
		want string
	}{
		{name: "threshold conflict", opt: listOptions{receivers: []string{address.String()}, threshold: 1}, want: "conflicts"},
		{name: "mixed receivers", opt: listOptions{receivers: []string{address.String(), "member"}}, want: "only receiver"},
		{name: "malformed address", opt: listOptions{receivers: []string{"MIX-invalid"}}, want: "invalid multisig receiver address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := normalizeListOptions(&tt.opt)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
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

func TestListSafeOutputsWithoutLimitReturnsAll(t *testing.T) {
	firstPage := make([]*mixin.SafeUtxo, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.SafeUtxo{Sequence: uint64(i + 1)}
	}
	secondPage := []*mixin.SafeUtxo{{Sequence: 501}, {Sequence: 502}}

	for _, order := range []string{"ASC", "DESC"} {
		t.Run(order, func(t *testing.T) {
			client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{firstPage, secondPage}}
			outputs, err := listSafeOutputs(context.Background(), client, listOptions{order: order})
			if err != nil {
				t.Fatal(err)
			}
			if len(outputs) != 502 {
				t.Fatalf("output count = %d, want 502", len(outputs))
			}
			if order == "ASC" && (outputs[0].Sequence != 1 || outputs[501].Sequence != 502) {
				t.Fatalf("unexpected ASC bounds: %d..%d", outputs[0].Sequence, outputs[501].Sequence)
			}
			if order == "DESC" && (outputs[0].Sequence != 502 || outputs[501].Sequence != 1) {
				t.Fatalf("unexpected DESC bounds: %d..%d", outputs[0].Sequence, outputs[501].Sequence)
			}
		})
	}
}

func TestListSafeOutputsLimitCanExceedAPIPageSize(t *testing.T) {
	firstPage := make([]*mixin.SafeUtxo, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.SafeUtxo{Sequence: uint64(i + 1)}
	}
	secondPage := []*mixin.SafeUtxo{{Sequence: 501}, {Sequence: 502}}

	for _, order := range []string{"ASC", "DESC"} {
		t.Run(order, func(t *testing.T) {
			client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{firstPage, secondPage}}
			outputs, err := listSafeOutputs(context.Background(), client, listOptions{limit: 501, order: order})
			if err != nil {
				t.Fatal(err)
			}
			if len(outputs) != 501 {
				t.Fatalf("output count = %d, want 501", len(outputs))
			}
			if order == "ASC" && outputs[len(outputs)-1].Sequence != 501 {
				t.Fatalf("ASC last sequence = %d, want 501", outputs[len(outputs)-1].Sequence)
			}
			if order == "DESC" && (outputs[0].Sequence != 502 || outputs[len(outputs)-1].Sequence != 2) {
				t.Fatalf("unexpected DESC bounds: %d..%d", outputs[0].Sequence, outputs[len(outputs)-1].Sequence)
			}
		})
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

func TestListSafeOutputsDescendingHonorsZeroOffset(t *testing.T) {
	client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{{{Sequence: 1}}}}

	outputs, err := listSafeOutputs(context.Background(), client, listOptions{
		offset: "0",
		order:  "DESC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 0 {
		t.Fatalf("output count = %d, want 0", len(outputs))
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

func TestListSafeOutputsAscendingRejectsStalledCursor(t *testing.T) {
	page := make([]*mixin.SafeUtxo, 500)
	for i := range page {
		page[i] = &mixin.SafeUtxo{Sequence: uint64(i + 1)}
	}
	client := &fakeSafeOutputClient{pages: [][]*mixin.SafeUtxo{page, page}}

	_, err := listSafeOutputs(context.Background(), client, listOptions{order: "ASC"})
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
		asset: "asset-a",
		limit: 3,
		order: "DESC",
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
		asset: "asset-a",
		limit: 2,
		order: "ASC",
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

func TestListLegacyOutputsWithoutLimitReturnsAll(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	page := []*mixin.MultisigUTXO{
		{UTXOID: "first", CreatedAt: start},
		{UTXOID: "second", CreatedAt: start.Add(time.Second)},
	}
	client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{page}}

	outputs, err := listLegacyOutputs(context.Background(), client, listOptions{order: "ASC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != len(page) {
		t.Fatalf("output count = %d, want %d", len(outputs), len(page))
	}
}

func TestListLegacyOutputsDoesNotSplitCreatedAtGroup(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		page []*mixin.MultisigUTXO
		want []string
	}{
		{
			name: "ASC",
			page: []*mixin.MultisigUTXO{
				{UTXOID: "a", CreatedAt: start},
				{UTXOID: "b", CreatedAt: start},
				{UTXOID: "newer", CreatedAt: start.Add(time.Second)},
			},
			want: []string{"a", "b"},
		},
		{
			name: "DESC",
			page: []*mixin.MultisigUTXO{
				{UTXOID: "older", CreatedAt: start},
				{UTXOID: "a", CreatedAt: start.Add(time.Second)},
				{UTXOID: "b", CreatedAt: start.Add(time.Second)},
			},
			want: []string{"b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{tt.page}}
			outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
				limit: 1,
				order: tt.name,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(outputs) != len(tt.want) {
				t.Fatalf("output count = %d, want %d", len(outputs), len(tt.want))
			}
			for i, output := range outputs {
				if output.UTXOID != tt.want[i] {
					t.Fatalf("output[%d] = %s, want %s", i, output.UTXOID, tt.want[i])
				}
			}
		})
	}
}

func TestListLegacyOutputsKeepsCreatedAtGroupAcrossAPIPages(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstPage := make([]*mixin.MultisigUTXO, 500)
	for i := range firstPage {
		firstPage[i] = &mixin.MultisigUTXO{
			UTXOID:    fmt.Sprintf("utxo-%d", i),
			CreatedAt: start.Add(time.Duration(i) * time.Second),
		}
	}
	boundary := firstPage[len(firstPage)-1].CreatedAt
	secondPage := []*mixin.MultisigUTXO{
		firstPage[len(firstPage)-1],
		{UTXOID: "same-time", CreatedAt: boundary},
		{UTXOID: "newer", CreatedAt: boundary.Add(time.Second)},
	}
	client := &fakeLegacyOutputLister{pages: [][]*mixin.MultisigUTXO{firstPage, secondPage}}

	outputs, err := listLegacyOutputs(context.Background(), client, listOptions{
		limit: 500,
		order: "ASC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 501 || outputs[len(outputs)-1].UTXOID != "same-time" {
		t.Fatalf("unexpected boundary group: count=%d last=%s", len(outputs), outputs[len(outputs)-1].UTXOID)
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
		asset: kernelAssetID,
		limit: 1,
		order: "ASC",
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
		asset: strings.Repeat("a", 64),
		limit: 1,
		order: "ASC",
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
		asset: "target-asset",
		limit: 500,
		order: "ASC",
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
	legacyOutputs, err := listLegacyOutputs(context.Background(), legacyClient, listOptions{limit: 1, order: "ASC"})
	if err != nil {
		t.Fatal(err)
	}
	if legacyOutputs == nil {
		t.Fatal("legacy outputs must be an empty array, not nil")
	}
}

func TestNewCmdListFlags(t *testing.T) {
	cmd := NewCmdList()
	for _, name := range []string{"receivers", "threshold", "asset", "state", "offset", "limit", "order"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
	if got := cmd.Flags().Lookup("limit").DefValue; got != "0" {
		t.Fatalf("default limit = %q, want 0", got)
	}
	if got := cmd.Flags().Lookup("order").DefValue; got != "ASC" {
		t.Fatalf("default order = %q, want ASC", got)
	}
	if usage := cmd.Flags().Lookup("receivers").Usage; !strings.Contains(usage, "MIX address") {
		t.Fatalf("receivers usage = %q, want MIX address support", usage)
	}
	if cmd.Flags().Lookup("legacy") != nil {
		t.Fatal("safe output list must not expose --legacy")
	}
}

func TestNewCmdLegacyListFlags(t *testing.T) {
	cmd := NewCmdLegacyList()
	for _, name := range []string{"receivers", "threshold", "asset", "state", "offset", "limit", "order"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
	if usage := cmd.Flags().Lookup("offset").Usage; !strings.Contains(usage, "RFC3339") {
		t.Fatalf("offset usage = %q, want RFC3339 timestamp", usage)
	}
	if cmd.Flags().Lookup("legacy") != nil {
		t.Fatal("legacy output list must not expose --legacy")
	}
}

func TestNewCmdLegacyContainsList(t *testing.T) {
	cmd := NewCmdLegacy()
	for _, subcommand := range cmd.Commands() {
		if subcommand.Name() == "list" {
			return
		}
	}
	t.Fatal("legacy command is missing list subcommand")
}

func TestNewCmdOutputSeparatesSafeAndLegacyLists(t *testing.T) {
	cmd := NewCmdOutput()

	safeList, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	if got := safeList.CommandPath(); got != "output list" {
		t.Fatalf("safe command path = %q, want output list", got)
	}

	legacyList, _, err := cmd.Find([]string{"legacy", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if got := legacyList.CommandPath(); got != "output legacy list" {
		t.Fatalf("legacy command path = %q, want output legacy list", got)
	}
}

func TestNewCmdOutputRejectsLegacyFlagOnSafeList(t *testing.T) {
	cmd := NewCmdOutput()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"list", "--legacy"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --legacy") {
		t.Fatalf("error = %v, want unknown --legacy flag", err)
	}
}
