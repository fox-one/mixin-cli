package cmdutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fox-one/mixin-sdk-go/v2"
)

func NormalizeMultisigDestination(input *mixin.TransferInput) error {
	if input == nil {
		return errors.New("transfer input is required")
	}
	return normalizeMultisigGroup(&input.OpponentMultisig.Receivers, &input.OpponentMultisig.Threshold, "receiver")
}

func normalizeMultisigGroup(members *[]string, threshold *uint8, role string) error {
	if members == nil || threshold == nil {
		return fmt.Errorf("multisig %s input is required", role)
	}
	values := *members
	if len(values) == 0 {
		return nil
	}

	addressIndex := -1
	for i, member := range values {
		if strings.HasPrefix(member, mixin.MixAddressPrefix) {
			addressIndex = i
			break
		}
	}
	if addressIndex < 0 {
		return nil
	}
	if len(values) != 1 {
		return fmt.Errorf("a multisig address must be the only %s", role)
	}

	address, err := mixin.MixAddressFromString(values[0])
	if err != nil {
		return fmt.Errorf("invalid multisig %s address: %w", role, err)
	}
	if *threshold != 0 && *threshold != address.Threshold {
		return fmt.Errorf("%s threshold %d conflicts with multisig address threshold %d", role, *threshold, address.Threshold)
	}
	*members = address.Members()
	*threshold = address.Threshold
	return nil
}
