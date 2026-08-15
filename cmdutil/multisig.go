package cmdutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fox-one/mixin-sdk-go/v2"
)

// NormalizeMultisigGroup expands a single MIX address into its members and threshold.
func NormalizeMultisigGroup(members *[]string, threshold *uint8) error {
	if members == nil || threshold == nil {
		return errors.New("multisig group input is required")
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
		return errors.New("a MIX address must be the only multisig receiver")
	}

	address, err := mixin.MixAddressFromString(values[0])
	if err != nil {
		return fmt.Errorf("invalid MIX address: %w", err)
	}
	if *threshold != 0 && *threshold != address.Threshold {
		return fmt.Errorf("threshold %d conflicts with MIX address threshold %d", *threshold, address.Threshold)
	}

	*members = address.Members()
	*threshold = address.Threshold
	return nil
}
