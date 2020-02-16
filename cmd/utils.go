package cmd

import (
	"errors"
	"os"
	"regexp"

	"github.com/chzyer/readline"
)

func getArg(args []string, idx int) (string, bool) {
	if idx < len(args) {
		return args[idx], true
	}

	return "", false
}

func validatePin(pin string) error {
	if match, _ := regexp.MatchString(`^\d{6}$`, pin); !match {
		return errors.New("pin must have 6 number characters exactly")
	}

	return nil
}

type stderr struct{}

func (s *stderr) Write(b []byte) (int, error) {
	if len(b) == 1 && b[0] == 7 {
		return 0, nil
	}
	return os.Stderr.Write(b)
}

func (s *stderr) Close() error {
	return os.Stderr.Close()
}

func init() {
	readline.Stdout = &stderr{}
}
