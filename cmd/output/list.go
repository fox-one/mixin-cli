package output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

type listOptions struct {
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

func NewCmdList() *cobra.Command {
	var opt listOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "list safe outputs",
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

			outputs, err := listSafeOutputs(ctx, client, opt)
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

	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "multisig members or one MIX address")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 0, "multisig threshold")
	cmd.Flags().StringVar(&opt.asset, "asset", "", "asset id or kernel asset id")
	cmd.Flags().StringVar(&opt.state, "state", "", "output state: unspent, signed, or spent")
	cmd.Flags().StringVar(&opt.offset, "offset", "", "safe sequence; exclusive upper bound for DESC")
	cmd.Flags().IntVar(&opt.limit, "limit", 0, "target number of outputs to return; 0 returns all")
	cmd.Flags().StringVar(&opt.order, "order", "ASC", "output order: ASC or DESC; DESC scans matching history client-side")
	return cmd
}

func normalizeListOptions(opt *listOptions) error {
	if opt == nil {
		return errors.New("output list options are required")
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = opt.receivers
	input.OpponentMultisig.Threshold = opt.threshold
	if err := cmdutil.NormalizeMultisigDestination(&input); err != nil {
		return err
	}
	opt.receivers = input.OpponentMultisig.Receivers
	opt.threshold = input.OpponentMultisig.Threshold
	return nil
}

func validateListOptions(opt listOptions) error {
	if len(opt.receivers) == 0 {
		if opt.threshold != 0 {
			return errors.New("receivers are required when threshold is set")
		}
	} else if opt.threshold == 0 || int(opt.threshold) > len(opt.receivers) {
		return errors.New("threshold must be in range [1, receivers count]")
	}

	if opt.limit < 0 {
		return errors.New("limit must not be negative")
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
		return listSafeOutputsAscending(ctx, client, query, opt.limit)
	}

	var before *uint64
	if opt.offset != "" {
		before = &offset
	}
	return listSafeOutputsDescending(ctx, client, query, before, opt.limit)
}

func listSafeOutputsAscending(ctx context.Context, client safeOutputClient, query mixin.SafeListUtxoOption, limit int) ([]*mixin.SafeUtxo, error) {
	const pageLimit = 500
	query.Limit = pageLimit

	result := make([]*mixin.SafeUtxo, 0)
	for {
		outputs, err := client.SafeListUtxos(ctx, query)
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 {
			break
		}

		for _, output := range outputs {
			result = append(result, output)
			if limit > 0 && len(result) == limit {
				return result, nil
			}
		}
		if len(outputs) < pageLimit {
			break
		}

		next := outputs[len(outputs)-1].Sequence + 1
		if next <= query.Offset {
			return nil, safePaginationStalledError(query.Offset)
		}
		query.Offset = next
	}
	return result, nil
}

func listSafeOutputsDescending(ctx context.Context, client safeOutputClient, query mixin.SafeListUtxoOption, before *uint64, limit int) ([]*mixin.SafeUtxo, error) {
	const pageLimit = 500
	query.Offset = 0
	query.Limit = pageLimit

	result := make([]*mixin.SafeUtxo, 0)
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
			if before != nil && output.Sequence >= *before {
				done = true
				break
			}
			result = append(result, output)
			if limit > 0 && len(result) > limit {
				result = result[len(result)-limit:]
			}
		}
		if done || len(outputs) < pageLimit {
			break
		}

		next := outputs[len(outputs)-1].Sequence + 1
		if next <= query.Offset {
			return nil, safePaginationStalledError(query.Offset)
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

func safePaginationStalledError(offset uint64) error {
	return fmt.Errorf("safe output pagination stalled at sequence %d", offset)
}
