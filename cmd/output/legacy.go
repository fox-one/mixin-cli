package output

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

type legacyOutputLister interface {
	ListMultisigOutputs(context.Context, mixin.ListMultisigOutputsOption) ([]*mixin.MultisigUTXO, error)
}

type legacyOutputClient interface {
	legacyOutputLister
	SafeReadAsset(context.Context, string) (*mixin.SafeAsset, error)
}

func NewCmdLegacy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "legacy",
		Short: "manage legacy multisig outputs",
	}

	cmd.AddCommand(NewCmdLegacyList())
	return cmd
}

func NewCmdLegacyList() *cobra.Command {
	var opt listOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "list legacy multisig outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := normalizeListOptions(&opt); err != nil {
				return err
			}
			if err := validateListOptions(opt); err != nil {
				return err
			}

			ctx := cmd.Context()
			client, err := session.From(ctx).GetClient()
			if err != nil {
				return err
			}

			outputs, err := listLegacyOutputs(ctx, client, opt)
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

	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "legacy multisig members or one MIX address")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 0, "legacy multisig threshold")
	cmd.Flags().StringVar(&opt.asset, "asset", "", "asset id or kernel asset id")
	cmd.Flags().StringVar(&opt.state, "state", "", "output state: unspent, signed, or spent")
	cmd.Flags().StringVar(&opt.offset, "offset", "", "RFC3339 timestamp; exclusive upper bound for DESC")
	cmd.Flags().IntVar(&opt.limit, "limit", 0, "target number of outputs to return; 0 returns all")
	cmd.Flags().StringVar(&opt.order, "order", "ASC", "output order: ASC or DESC; DESC scans matching history client-side")
	return cmd
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
	var cursor *time.Time
	if opt.offset != "" {
		cursor = &offset
	}
	if strings.EqualFold(opt.order, "DESC") {
		return listLegacyOutputsDescending(ctx, client, query, cursor, assetID, opt.limit)
	}
	return listLegacyOutputsAscending(ctx, client, query, assetID, opt.limit, cursor)
}

func listLegacyOutputsAscending(ctx context.Context, client legacyOutputLister, query mixin.ListMultisigOutputsOption, assetID string, limit int, after *time.Time) ([]*mixin.MultisigUTXO, error) {
	const pageLimit = 500
	query.Limit = pageLimit

	result := make([]*mixin.MultisigUTXO, 0)
	seen := map[string]struct{}{}
	var cutoff *time.Time
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
			if after != nil && !output.CreatedAt.After(*after) {
				continue
			}
			if cutoff != nil && output.CreatedAt.After(*cutoff) {
				done = true
				break
			}
			if isDuplicateLegacyOutput(seen, output) {
				continue
			}
			if assetID != "" && output.AssetID != assetID {
				continue
			}
			result = append(result, output)
			if limit > 0 && len(result) == limit {
				value := output.CreatedAt
				cutoff = &value
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
	return result, nil
}

func listLegacyOutputsDescending(ctx context.Context, client legacyOutputLister, query mixin.ListMultisigOutputsOption, before *time.Time, assetID string, limit int) ([]*mixin.MultisigUTXO, error) {
	const pageLimit = 500
	query.Offset = time.Time{}
	query.Limit = pageLimit

	result := make([]*mixin.MultisigUTXO, 0)
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
			if before != nil && !output.CreatedAt.Before(*before) {
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
		}
		if done || pageSize < pageLimit {
			break
		}
		if !next.After(query.Offset) {
			return nil, legacyPaginationStalledError(query.Offset)
		}
		query.Offset = next
	}

	if limit > 0 && len(result) > limit {
		start := len(result) - limit
		cutoff := result[start].CreatedAt
		for start > 0 && result[start-1].CreatedAt.Equal(cutoff) {
			start--
		}
		result = result[start:]
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
