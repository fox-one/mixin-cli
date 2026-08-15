package cmdutil

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
)

func TestNormalizeMultisigGroupExpandsMixAddress(t *testing.T) {
	members := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}
	address, err := mixin.NewMixAddress(members, 2)
	if err != nil {
		t.Fatal(err)
	}

	values := []string{address.String()}
	var threshold uint8
	if err := NormalizeMultisigGroup(&values, &threshold); err != nil {
		t.Fatal(err)
	}
	if threshold != 2 {
		t.Fatalf("threshold = %d, want 2", threshold)
	}
	assertMembersEqual(t, values, members)
}

func TestNormalizeMultisigGroupExpandsMainnetMixAddress(t *testing.T) {
	members := []string{
		mixinnet.GenerateAddress(rand.Reader, true).String(),
		mixinnet.GenerateAddress(rand.Reader, true).String(),
	}
	address, err := mixin.NewMainnetMixAddress(members, 1)
	if err != nil {
		t.Fatal(err)
	}

	values := []string{address.String()}
	var threshold uint8
	if err := NormalizeMultisigGroup(&values, &threshold); err != nil {
		t.Fatal(err)
	}
	if threshold != 1 {
		t.Fatalf("threshold = %d, want 1", threshold)
	}
	assertMembersEqual(t, values, members)
}

func TestNormalizeMultisigGroupAcceptsMatchingThreshold(t *testing.T) {
	address, err := mixin.NewMixAddress([]string{"00000000-0000-0000-0000-000000000001"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	values := []string{address.String()}
	threshold := uint8(1)
	if err := NormalizeMultisigGroup(&values, &threshold); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeMultisigGroupKeepsExplicitMembers(t *testing.T) {
	want := []string{"member-a", "member-b"}
	values := append([]string(nil), want...)
	threshold := uint8(2)

	if err := NormalizeMultisigGroup(&values, &threshold); err != nil {
		t.Fatal(err)
	}
	if threshold != 2 {
		t.Fatalf("threshold = %d, want 2", threshold)
	}
	assertMembersEqual(t, values, want)
}

func TestNormalizeMultisigGroupRejectsInvalidMixAddressInput(t *testing.T) {
	address, err := mixin.NewMixAddress([]string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
	}, 2)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		values    []string
		threshold uint8
		want      string
	}{
		{name: "threshold conflict", values: []string{address.String()}, threshold: 1, want: "conflicts"},
		{name: "mixed receivers", values: []string{address.String(), "member"}, want: "only multisig receiver"},
		{name: "malformed address", values: []string{"MIX-invalid"}, want: "invalid MIX address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := append([]string(nil), tt.values...)
			threshold := tt.threshold
			err := NormalizeMultisigGroup(&values, &threshold)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func assertMembersEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("members = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member %d = %q, want %q", i, got[i], want[i])
		}
	}
}
