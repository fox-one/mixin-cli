package safe

import (
	"crypto/rand"
	"fmt"

	"github.com/fox-one/mixin-cli/cmdutil"
	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

func NewCmdMigrate() *cobra.Command {
	var opt struct {
		yes bool
	}

	cmd := &cobra.Command{
		Use:   "migrate <spend-key>",
		Short: "migrate to safe network",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			var spendKey string
			if len(args) > 0 {
				spendKey = args[0]
			} else {
				spendKey = mixinnet.GenerateKey(rand.Reader).String()
			}
			cmd.Println("spend key:", spendKey)

			pin, err := cmdutil.GetOrReadPin(s)
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}

			if !opt.yes && !conformContinue() {
				return nil
			}

			user, err := client.SafeMigrate(ctx, spendKey, pin)
			if err != nil {
				return fmt.Errorf("migrate failed: %w", err)
			}

			if user.HasSafe {
				cmd.Println("migrate success!")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opt.yes, "yes", false, "approve migrate automatically")
	return cmd
}
