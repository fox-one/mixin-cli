package list

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/fox-one/mixin-cli/v2/pkg/column"
	"github.com/fox-one/mixin-cli/v2/pkg/jq"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdList() *cobra.Command {
	var opt struct {
		input mixin.TransferInput
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "list assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			var assets []*mixin.Asset
			if len(opt.input.OpponentMultisig.Receivers) > 0 || opt.input.OpponentMultisig.Threshold > 0 {
				if err := validateLegacyMultisigGroup(opt.input.OpponentMultisig.Receivers, opt.input.OpponentMultisig.Threshold); err != nil {
					return err
				}
				assets, err = readLegacyMultisigAssets(ctx, client, opt.input)
			} else {
				assets, err = client.ReadAssets(ctx)
			}
			if err != nil {
				return err
			}

			// filter assets with positive balance
			var (
				idx        int
				totalValue decimal.Decimal
			)

			for _, asset := range assets {
				if !asset.Balance.IsPositive() {
					continue
				}

				totalValue = totalValue.Add(asset.PriceUSD.Mul(asset.Balance))
				assets[idx] = asset
				idx++
			}
			assets = assets[:idx]

			data, _ := json.Marshal(assets)
			fields := []string{"asset_id", "symbol", "name", "balance"}
			for _, arg := range args {
				if !govalidator.IsIn(arg, fields...) {
					fields = append(fields, arg)
				}
			}

			lines, err := jq.ParseObjects(data, fields...)
			if err != nil {
				return err
			}

			fmt.Println(column.Print(lines))
			fmt.Println("Total USD Value:", totalValue)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&opt.input.OpponentMultisig.Receivers, "receivers", nil, "legacy multisig receivers")
	cmd.Flags().Uint8Var(&opt.input.OpponentMultisig.Threshold, "threshold", 0, "legacy multisig threshold")

	return cmd
}

type legacyMultisigOutputLister interface {
	ListMultisigOutputs(context.Context, mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error)
}

type safeAssetFetcher interface {
	SafeFetchAssets(context.Context, []string) ([]*mixin.SafeAsset, error)
	SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error)
}

type legacyMultisigClient interface {
	legacyMultisigOutputLister
	safeAssetFetcher
}

func readLegacyMultisigAssets(ctx context.Context, client legacyMultisigClient, input mixin.TransferInput) ([]*mixin.Asset, error) {
	balances, err := readLegacyMultisigBalances(ctx, client, input.OpponentMultisig.Receivers, input.OpponentMultisig.Threshold)
	if err != nil {
		return nil, err
	}
	if len(balances) == 0 {
		return []*mixin.Asset{}, nil
	}

	assetIDs := make([]string, 0, len(balances))
	for assetID := range balances {
		assetIDs = append(assetIDs, assetID)
	}
	sort.Strings(assetIDs)

	assets, err := client.SafeFetchAssets(ctx, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch legacy multisig asset metadata: %w", err)
	}
	assetMap := make(map[string]*mixin.SafeAsset, len(assets))
	for _, asset := range assets {
		assetMap[asset.AssetID] = asset
	}
	for _, assetID := range assetIDs {
		if assetMap[assetID] != nil {
			continue
		}
		asset, err := client.SafeReadAsset(ctx, assetID)
		if err != nil {
			return nil, fmt.Errorf("read legacy multisig asset metadata %s: %w", assetID, err)
		}
		assetMap[assetID] = asset
	}

	result := make([]*mixin.Asset, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		asset := &mixin.Asset{AssetID: assetID, Symbol: "-", Name: "-"}
		if safeAsset := assetMap[assetID]; safeAsset != nil {
			asset.ChainID = safeAsset.ChainID
			asset.AssetKey = safeAsset.AssetKey
			asset.Symbol = safeAsset.Symbol
			asset.Name = safeAsset.Name
			asset.IconURL = safeAsset.IconURL
			asset.PriceBTC = safeAsset.PriceBTC
			asset.PriceUSD = safeAsset.PriceUSD
			asset.ChangeBTC = safeAsset.ChangeBTC
			asset.ChangeUsd = safeAsset.ChangeUsd
			asset.Confirmations = safeAsset.Confirmations
		}
		asset.Balance = balances[assetID]
		result = append(result, asset)
	}
	return result, nil
}

func readLegacyMultisigBalances(ctx context.Context, client legacyMultisigOutputLister, receivers []string, threshold uint8) (map[string]decimal.Decimal, error) {
	balances := map[string]decimal.Decimal{}
	seen := map[string]struct{}{}
	offset := time.Time{}
	const limit = 500

	for {
		outputs, err := client.ListMultisigOutputs(ctx, mixin.ListMultisigOutputsOption{
			Members:        receivers,
			Threshold:      threshold,
			Offset:         offset,
			Limit:          limit,
			OrderByCreated: true,
			State:          mixin.UTXOStateUnspent,
		})
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 {
			break
		}

		pageSize := len(outputs)
		nextOffset := outputs[len(outputs)-1].CreatedAt

		for _, output := range outputs {
			if output.UTXOID != "" {
				if _, ok := seen[output.UTXOID]; ok {
					continue
				}
				seen[output.UTXOID] = struct{}{}
			}
			balances[output.AssetID] = balances[output.AssetID].Add(output.Amount)
		}

		if pageSize < limit {
			break
		}
		if !nextOffset.After(offset) {
			return nil, fmt.Errorf("legacy output pagination stalled at %s", offset.UTC().Format(time.RFC3339Nano))
		}
		offset = nextOffset
	}
	return balances, nil
}

func validateLegacyMultisigGroup(receivers []string, threshold uint8) error {
	if len(receivers) == 0 {
		return errors.New("receivers are required when threshold is set")
	}
	if threshold == 0 || int(threshold) > len(receivers) {
		return errors.New("threshold must be in range [1, receivers count]")
	}
	return nil
}
