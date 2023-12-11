package safe

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func NewCmdSafe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "safe",
		Short: "safe",
	}

	cmd.AddCommand(NewCmdMigrate())
	cmd.AddCommand(NewCmdAssets())
	cmd.AddCommand(NewCmdTransfer())
	cmd.AddCommand(NewCmdDeposit())
	cmd.AddCommand(NewCmdListDeposit())
	cmd.AddCommand(NewCmdMixAddress())
	return cmd
}

func conformContinue() bool {
	prompt := promptui.Prompt{
		Label:     "Continue",
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if err != nil {
		return false
	}

	switch result {
	case "y", "Y":
		return true
	default:
		return false
	}
}
