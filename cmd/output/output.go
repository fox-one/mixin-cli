package output

import "github.com/spf13/cobra"

func NewCmdOutput() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "output",
		Short: "manage safe and legacy outputs",
	}

	cmd.AddCommand(NewCmdList())
	return cmd
}
