package ownership

import (
	"fmt"

	"github.com/gofrs/uuid"

	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/spf13/cobra"
)

func NewCmdTransfer() *cobra.Command {
	var opt struct {
		UserID string
	}

	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "transfer ownership",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			if uuid.FromStringOrNil(opt.UserID).IsNil() {
				return fmt.Errorf("invalid user_id: %s", opt.UserID)
			}

			pin, err := cmdutil.GetOrReadPin(s)
			if err != nil {
				return fmt.Errorf("read pin failed: %w", err)
			}

			return client.TransferOwnership(ctx, opt.UserID, pin)
		},
	}

	cmd.Flags().StringVar(&opt.UserID, "user_id", "", "user id")

	return cmd
}
