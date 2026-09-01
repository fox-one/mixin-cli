package safe

import (
	"context"
	"errors"
	"fmt"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-sdk-go/v2"
)

func listUnspentOutputs(ctx context.Context, client safeUtxoLister, members []string, threshold uint8) (map[string][]*mixin.SafeUtxo, error) {
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = members
	input.OpponentMultisig.Threshold = threshold
	if err := cmdutil.NormalizeMultisigDestination(&input); err != nil {
		return nil, err
	}
	members = input.OpponentMultisig.Receivers
	threshold = input.OpponentMultisig.Threshold
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
		if len(items) < LIMIT {
			return result, nil
		}

		next := items[len(items)-1].Sequence + 1
		if next <= offset {
			return nil, fmt.Errorf("safe output pagination stalled at sequence %d", offset)
		}
		offset = next
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
