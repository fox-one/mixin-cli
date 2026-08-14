package safe

import (
	"context"
	"errors"

	"github.com/fox-one/mixin-sdk-go/v2"
)

type safeUtxoLister interface {
	SafeListUtxos(context.Context, mixin.SafeListUtxoOption) ([]*mixin.SafeUtxo, error)
}

func listAssetUnspentOutputs(ctx context.Context, client *mixin.Client, asset string) ([]*mixin.SafeUtxo, error) {
	var result []*mixin.SafeUtxo

	const LIMIT = 256
	var offset uint64
	for {
		items, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Offset: offset,
			State:  mixin.SafeUtxoStateUnspent,
			Asset:  asset,
			Limit:  LIMIT,
		})
		if err != nil {
			return nil, err
		}

		result = append(result, items...)
		if len(items) > 0 {
			offset = items[len(items)-1].Sequence + 1
		}

		if len(items) < LIMIT {
			return result, nil
		}
	}
}

func listUnspentOutputs(ctx context.Context, client safeUtxoLister, members []string, threshold uint8) (map[string][]*mixin.SafeUtxo, error) {
	if err := validateMultisigGroup(members, threshold); err != nil {
		return nil, err
	}

	result := map[string][]*mixin.SafeUtxo{}

	const LIMIT = 256
	var offset uint64
	for {
		items, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Members:   members,
			Threshold: threshold,
			Offset:    offset,
			State:     mixin.SafeUtxoStateUnspent,
			Limit:     LIMIT,
			Order:     "ASC",
		})
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			result[item.AssetID] = append(result[item.AssetID], item)
		}
		if len(items) > 0 {
			offset = items[len(items)-1].Sequence + 1
		}

		if len(items) < LIMIT {
			return result, nil
		}
	}
}

func validateMultisigGroup(members []string, threshold uint8) error {
	if len(members) == 0 {
		if threshold != 0 {
			return errors.New("receivers are required when threshold is set")
		}
		return nil
	}

	if threshold == 0 || int(threshold) > len(members) {
		return errors.New("threshold must be in range [1, receivers count]")
	}
	return nil
}
