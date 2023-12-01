package safe

import (
	"context"

	"github.com/fox-one/mixin-sdk-go/v2"
)

func listAssetUnspentOutputs(ctx context.Context, client *mixin.Client, asset string) ([]*mixin.SafeUtxo, error) {
	var result []*mixin.SafeUtxo

	const LIMIT = 500
	var offset uint64
	for {
		items, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Offset: offset,
			State:  mixin.SafeUtxoStateUnspent,
			Asset:  asset,
			Limit:  500,
		})
		if err != nil {
			return nil, err
		}

		result = append(result, items...)
		if len(items) > 0 {
			offset = items[len(items)-1].Sequence
		}

		if len(items) < LIMIT {
			return result, nil
		}
	}
}

func listUnspentOutputs(ctx context.Context, client *mixin.Client) (map[string][]*mixin.SafeUtxo, error) {
	var result = map[string][]*mixin.SafeUtxo{}

	const LIMIT = 500
	var offset uint64
	for {
		items, err := client.SafeListUtxos(ctx, mixin.SafeListUtxoOption{
			Offset: offset,
			State:  mixin.SafeUtxoStateUnspent,
			Limit:  500,
		})
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			result[item.Asset.String()] = append(result[item.Asset.String()], item)
			offset = items[len(items)-1].Sequence
		}

		if len(items) < LIMIT {
			return result, nil
		}
	}
}
