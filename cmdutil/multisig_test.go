package cmdutil

import (
	"crypto/rand"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
)

func TestNormalizeMultisigDestinationExpandsAddress(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	address, err := mixin.NewMixAddress(members, 2)
	if err != nil {
		t.Fatal(err)
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{address.String()}

	if err := NormalizeMultisigDestination(&input); err != nil {
		t.Fatal(err)
	}
	if input.OpponentMultisig.Threshold != 2 {
		t.Fatalf("threshold = %d, want 2", input.OpponentMultisig.Threshold)
	}
	if len(input.OpponentMultisig.Receivers) != len(members) {
		t.Fatalf("receivers = %#v, want %#v", input.OpponentMultisig.Receivers, members)
	}
	for i := range members {
		if input.OpponentMultisig.Receivers[i] != members[i] {
			t.Fatalf("receiver %d = %q, want %q", i, input.OpponentMultisig.Receivers[i], members[i])
		}
	}
}

func TestNormalizeMultisigDestinationAcceptsMatchingThreshold(t *testing.T) {
	members := []string{"00000000-0000-0000-0000-000000000001"}
	address, err := mixin.NewMixAddress(members, 1)
	if err != nil {
		t.Fatal(err)
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{address.String()}
	input.OpponentMultisig.Threshold = 1
	if err := NormalizeMultisigDestination(&input); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeMultisigDestinationExpandsMainnetAddress(t *testing.T) {
	members := []string{
		mixinnet.GenerateAddress(rand.Reader, true).String(),
		mixinnet.GenerateAddress(rand.Reader, true).String(),
	}
	address, err := mixin.NewMainnetMixAddress(members, 1)
	if err != nil {
		t.Fatal(err)
	}
	input := mixin.TransferInput{}
	input.OpponentMultisig.Receivers = []string{address.String()}
	if err := NormalizeMultisigDestination(&input); err != nil {
		t.Fatal(err)
	}
	if input.OpponentMultisig.Threshold != 1 {
		t.Fatalf("threshold = %d, want 1", input.OpponentMultisig.Threshold)
	}
	for i := range members {
		if input.OpponentMultisig.Receivers[i] != members[i] {
			t.Fatalf("receiver %d = %q, want %q", i, input.OpponentMultisig.Receivers[i], members[i])
		}
	}
}

func TestNormalizeMultisigDestinationRejectsConflicts(t *testing.T) {
	address, err := mixin.NewMixAddress([]string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}, 2)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("threshold", func(t *testing.T) {
		input := mixin.TransferInput{}
		input.OpponentMultisig.Receivers = []string{address.String()}
		input.OpponentMultisig.Threshold = 1
		if err := NormalizeMultisigDestination(&input); err == nil {
			t.Fatal("expected threshold conflict")
		}
	})

	t.Run("mixed receivers", func(t *testing.T) {
		input := mixin.TransferInput{}
		input.OpponentMultisig.Receivers = []string{address.String(), "00000000-0000-0000-0000-000000000003"}
		if err := NormalizeMultisigDestination(&input); err == nil {
			t.Fatal("expected mixed receiver conflict")
		}
	})

	t.Run("malformed address", func(t *testing.T) {
		input := mixin.TransferInput{}
		input.OpponentMultisig.Receivers = []string{"MIX-invalid"}
		if err := NormalizeMultisigDestination(&input); err == nil {
			t.Fatal("expected malformed address error")
		}
	})
}

func TestNormalizeMultisigSourceExpandsAddress(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	address, err := mixin.NewMixAddress(members, 2)
	if err != nil {
		t.Fatal(err)
	}
	senders := []string{address.String()}
	var threshold uint8
	if err := NormalizeMultisigSource(&senders, &threshold); err != nil {
		t.Fatal(err)
	}
	if threshold != 2 {
		t.Fatalf("threshold = %d, want 2", threshold)
	}
	for i := range members {
		if senders[i] != members[i] {
			t.Fatalf("sender %d = %q, want %q", i, senders[i], members[i])
		}
	}
}

func TestNormalizeMultisigSourceRejectsThresholdConflict(t *testing.T) {
	address, err := mixin.NewMixAddress([]string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	senders := []string{address.String()}
	threshold := uint8(1)
	if err := NormalizeMultisigSource(&senders, &threshold); err == nil {
		t.Fatal("expected sender threshold conflict")
	}
}
