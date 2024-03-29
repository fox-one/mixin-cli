package cmdutil

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"syscall"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"golang.org/x/term"
)

func UserMe(ctx context.Context, s *session.Session) (*mixin.User, error) {
	keystore, err := s.GetKeystore()
	if err != nil {
		return nil, err
	}
	client, err := mixin.NewFromKeystore(keystore)
	if err != nil {
		return nil, err
	}
	return client.UserMe(context.Background())
}

func DecodeBase64(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.StdEncoding.DecodeString(s)
	}

	return b, err
}

func GetOrReadPin(s *session.Session) (string, error) {
	pin := s.GetPin()
	if pin != "" {
		return pin, nil
	}

	var user *mixin.User
	for {
		fmt.Print("Enter PIN: ")
		inputData, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", err
		}
		fmt.Println()
		pin = string(inputData)
		if pin != "" {
			if len(pin) > 6 {
				if user == nil {
					user, err = UserMe(context.Background(), s)
					if err != nil {
						return "", err
					}
				}

				if user.TipKeyBase64 != "" {
					tipPub := user.TipKeyBase64
					if tipPubBts, err := DecodeBase64(tipPub); err == nil {
						tipPub = hex.EncodeToString(tipPubBts)
					}
					pinKey, err := mixinnet.ParseKeyWithPub(pin, tipPub)
					if err != nil {
						return "", err
					}
					pin = pinKey.String()
				}
			}
			s.WithPin(pin)
			return pin, nil
		}
	}
}

func GetOrSpendKey(s *session.Session) (*mixinnet.Key, error) {
	spendKey := s.GetSpendKey()
	if spendKey != nil {
		return spendKey, nil
	}

	var user *mixin.User
	for {
		fmt.Print("Enter PIN: ")
		inputData, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, err
		}
		fmt.Println()
		if user == nil {
			user, err = UserMe(context.Background(), s)
			if err != nil {
				return nil, err
			}
			if user.SpendPublicKey == "" {
				return nil, fmt.Errorf("user has no spend key")
			}
		}
		spendKey, err := mixinnet.ParseKeyWithPub(string(inputData), user.SpendPublicKey)
		if err != nil {
			return nil, err
		}
		if spendKey.HasValue() {
			s.WithSpendKey(&spendKey)
			return &spendKey, nil
		}
	}
}
