package output

import "github.com/spf13/cobra"

func NewCmdOutput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "output",
		Short: "manage outputs",
	}

	cmd.AddCommand(NewCmdList())
	cmd.AddCommand(NewCmdLegacy())
	return cmd
}
