package cmd

import (
	"os"

	"github.com/chzyer/readline"
)

func getArg(args []string, idx int) (string, bool) {
	if idx < len(args) {
		return args[idx], true
	}

	return "", false
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
