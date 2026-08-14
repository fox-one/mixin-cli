package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

type listOptions struct {
	legacy    bool
	receivers []string
	threshold uint8
	asset     string
	state     string
	offset    string
	limit     int
	order     string
}

type safeOutputClient interface {
	SafeListUtxos(context.Context, mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error)
	SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error)
}

type legacyOutputLister interface {
	ListMultisigOutputs(context.Context, mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error)
}

type legacyOutputClient interface {
	legacyOutputLister
	SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error)
}

func NewCmdList() *cobra.Command {
	var opt listOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "list safe or legacy outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateListOptions(opt); err != nil {
				return err
			}

			ctx := cmd.Context()
			client, err := session.From(ctx).GetClient()
			if err != nil {
				return err
			}

			var outputs any
			if opt.legacy {
				outputs, err = listLegacyOutputs(ctx, client, opt)
			} else {
				outputs, err = listSafeOutputs(ctx, client, opt)
			}
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(outputs, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().BoolVar(&opt.legacy, "legacy", false, "list legacy multisig outputs")
	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "multisig receivers")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 0, "multisig threshold")
	cmd.Flags().StringVar(&opt.asset, "asset", "", "asset id or kernel asset id")
	cmd.Flags().StringVar(&opt.state, "state", "", "output state: unspent, signed, or spent")
	cmd.Flags().StringVar(&opt.offset, "offset", "", "safe sequence or legacy RFC3339 timestamp; exclusive upper bound for DESC")
	cmd.Flags().IntVar(&opt.limit, "limit", 100, "number of outputs to return")
	cmd.Flags().StringVar(&opt.order, "order", "ASC", "output order: ASC or DESC; DESC scans matching history client-side")
	return cmd
}

func validateListOptions(opt listOptions) error {
	if len(opt.receivers) == 0 {
		if opt.threshold != 0 {
			return errors.New("receivers are required when threshold is set")
		}
	} else if opt.threshold == 0 || int(opt.threshold) > len(opt.receivers) {
		return errors.New("threshold must be in range [1, receivers count]")
	}

	if opt.limit < 1 || opt.limit > 500 {
		return errors.New("limit must be in range [1, 500]")
	}

	switch opt.state {
	case "", mixin.UTXOStateUnspent, mixin.UTXOStateSigned, mixin.UTXOStateSpent:
	default:
		return errors.New("state must be one of unspent, signed, or spent")
	}

	switch strings.ToUpper(opt.order) {
	case "ASC", "DESC":
	default:
		return errors.New("order must be ASC or DESC")
	}

	return nil
}

func listSafeOutputs(ctx context.Context, client safeOutputClient, opt listOptions) ([]*mixin.SafeUtxo, error) {
	var offset uint64
	if opt.offset != "" {
		value, err := strconv.ParseUint(opt.offset, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse safe offset: %w", err)
		}
		offset = value
	}

	asset := opt.asset
	if asset != "" {
		if _, err := mixinnet.HashFromString(asset); err != nil {
			safeAsset, err := client.SafeReadAsset(ctx, asset)
			if err != nil {
				return nil, fmt.Errorf("read safe asset: %w", err)
			}
			asset = safeAsset.KernelAssetID
		}
	}

	query := mixin.SafeListUtxoOption{
		Members:   opt.receivers,
		Threshold: opt.threshold,
		Offset:    offset,
		Asset:     asset,
		Limit:     opt.limit,
		Order:     "ASC",
		State:     mixin.SafeUtxoState(opt.state),
	}
	if strings.EqualFold(opt.order, "ASC") {
		outputs, err := client.SafeListUtxos(ctx, query)
		if outputs == nil {
			outputs = []*mixin.SafeUtxo{}
		}
		return outputs, err
	}

	return listSafeOutputsDescending(ctx, client, query, offset, opt.limit)
}

func listSafeOutputsDescending(ctx context.Context, client safeOutputClient, query mixin.SafeListUtxoOption, before uint64, limit int) ([]*mixin.SafeUtxo, error) {
	const pageLimit = 500
	query.Offset = 0
	query.Limit = pageLimit

	result := make([]*mixin.SafeUtxo, 0, limit)
	for {
		outputs, err := client.SafeListUtxos(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 {
			break
		}

		done := false
		for _, output := range outputs {
			if before > 0 && output.Sequence >= before {
				done = true
				break
			}
			result = append(result, output)
			if len(result) > limit {
				result = result[len(result)-limit:]
			}
		}
		if done || len(outputs) < pageLimit {
			break
		}

		next := outputs[len(outputs)-1].Sequence + 1
		if next <= query.Offset {
			break
		}
		query.Offset = next
	}

	reverseSafeOutputs(result)
	return result, nil
}

func reverseSafeOutputs(outputs []*mixin.SafeUtxo) {
	for i, j := 0, len(outputs)-1; i < j; i, j = i+1, j-1 {
		outputs[i], outputs[j] = outputs[j], outputs[i]
	}
}

func listLegacyOutputs(ctx context.Context, client legacyOutputClient, opt listOptions) ([]*mixin.MultisigUTXO, error) {
	var offset time.Time
	if opt.offset != "" {
		value, err := time.Parse(time.RFC3339Nano, opt.offset)
		if err != nil {
			return nil, fmt.Errorf("parse legacy offset: %w", err)
		}
		offset = value
	}

	assetID := opt.asset
	if assetID != "" {
		if _, err := mixinnet.HashFromString(assetID); err == nil {
			asset, err := client.SafeReadAsset(ctx, assetID)
			if err != nil {
				return nil, fmt.Errorf("resolve legacy kernel asset: %w", err)
			}
			assetID = asset.AssetID
		}
	}

	query := mixin.ListMultisigOutputsOption{
		Members:        opt.receivers,
		Threshold:      opt.threshold,
		Offset:         offset,
		Limit:          opt.limit,
		OrderByCreated: true,
		State:          opt.state,
	}
	if strings.EqualFold(opt.order, "DESC") {
		return listLegacyOutputsDescending(ctx, client, query, offset, assetID, opt.limit)
	}
	if assetID != "" {
		return listLegacyOutputsAscending(ctx, client, query, assetID, opt.limit)
	}

	outputs, err := client.ListMultisigOutputs(ctx, query)
	if err != nil {
		return nil, err
	}
	if outputs == nil {
		outputs = []*mixin.MultisigUTXO{}
	}
	return outputs, nil
}

func listLegacyOutputsAscending(ctx context.Context, client legacyOutputLister, query mixin.ListMultisigOutputsOption, assetID string, limit int) ([]*mixin.MultisigUTXO, error) {
	const pageLimit = 500
	query.Limit = pageLimit

	result := make([]*mixin.MultisigUTXO, 0, limit)
	seen := map[string]struct{}{}
	for len(result) < limit {
		outputs, err := client.ListMultisigOutputs(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 {
			break
		}

		pageSize := len(outputs)
		next := outputs[len(outputs)-1].CreatedAt
		for _, output := range outputs {
			if isDuplicateLegacyOutput(seen, output) {
				continue
			}
			if output.AssetID == assetID {
				result = append(result, output)
				if len(result) == limit {
					break
				}
			}
		}
		if pageSize < pageLimit {
			break
		}
		if !next.After(query.Offset) {
			return nil, legacyPaginationStalledError(query.Offset)
		}
		query.Offset = next
	}
	return result, nil
}

func listLegacyOutputsDescending(ctx context.Context, client legacyOutputLister, query mixin.ListMultisigOutputsOption, before time.Time, assetID string, limit int) ([]*mixin.MultisigUTXO, error) {
	const pageLimit = 500
	query.Offset = time.Time{}
	query.Limit = pageLimit

	result := make([]*mixin.MultisigUTXO, 0, limit)
	seen := map[string]struct{}{}
	for {
		outputs, err := client.ListMultisigOutputs(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 {
			break
		}

		pageSize := len(outputs)
		next := outputs[len(outputs)-1].CreatedAt

		done := false
		for _, output := range outputs {
			if !before.IsZero() && !output.CreatedAt.Before(before) {
				done = true
				break
			}
			if assetID != "" && output.AssetID != assetID {
				continue
			}
			if isDuplicateLegacyOutput(seen, output) {
				continue
			}
			result = append(result, output)
			if len(result) > limit {
				result = result[len(result)-limit:]
			}
		}
		if done || pageSize < pageLimit {
			break
		}
		if !next.After(query.Offset) {
			return nil, legacyPaginationStalledError(query.Offset)
		}
		query.Offset = next
	}

	reverseLegacyOutputs(result)
	return result, nil
}

func isDuplicateLegacyOutput(seen map[string]struct{}, output *mixin.MultisigUTXO) bool {
	if output.UTXOID == "" {
		return false
	}
	if _, ok := seen[output.UTXOID]; ok {
		return true
	}
	seen[output.UTXOID] = struct{}{}
	return false
}

func legacyPaginationStalledError(offset time.Time) error {
	return fmt.Errorf("legacy output pagination stalled at %s", offset.UTC().Format(time.RFC3339Nano))
}

func reverseLegacyOutputs(outputs []*mixin.MultisigUTXO) {
	for i, j := 0, len(outputs)-1; i < j; i, j = i+1, j-1 {
		outputs[i], outputs[j] = outputs[j], outputs[i]
	}
}
