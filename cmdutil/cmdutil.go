package cmdutil

import (
	"fmt"
	"syscall"

	"github.com/fox-one/mixin-cli/session"
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
