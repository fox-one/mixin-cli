package cmdutil

import (
	"fmt"
	"syscall"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"golang.org/x/term"
)

func GetOrReadPin(s *session.Session) (string, error) {
	pin := s.GetPin()
	if pin != "" {
		return pin, nil
	}

	for {
		fmt.Print("Enter PIN: ")
		inputData, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", err
		}
		fmt.Println()
		pin = string(inputData)
		if pin != "" {
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

	for {
		fmt.Print("Enter PIN: ")
		inputData, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, err
		}
		fmt.Println()
		spendKey, err := mixinnet.KeyFromString(string(inputData))
		if err != nil {
			return nil, err
		}
		if spendKey.HasValue() {
			s.WithSpendKey(&spendKey)
			return &spendKey, nil
		}
	}
}
