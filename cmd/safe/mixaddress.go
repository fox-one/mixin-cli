package safe

import (
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/spf13/cobra"
)

func NewCmdMixAddress() *cobra.Command {
	var opt struct {
		receivers         []string
		threshold         uint8
		isMixinnetAddress bool
	}

	cmd := &cobra.Command{
		Use:   "mixaddress",
		Short: "generate mix address",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(opt.receivers) == 0 {
				cmd.Println("receivers is empty")
				return nil
			}

			if opt.threshold == 0 {
				opt.threshold = uint8(len(opt.receivers))
			}
			if opt.threshold > uint8(len(opt.receivers)) {
				cmd.Println("threshold is too large")
				return nil
			}

			addrFunc := mixin.NewMixAddress
			if opt.isMixinnetAddress {
				addrFunc = mixin.NewMainnetMixAddress
			}
			addr, err := addrFunc(opt.receivers, opt.threshold)
			if err != nil {
				cmd.Println("generate mix address failed:", err)
				return err
			}

			cmd.Println("address:", addr.String())
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "multisig receivers")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 0, "multisig threshold")
	cmd.Flags().BoolVar(&opt.isMixinnetAddress, "mixinnet", false, "mixinnet address")

	return cmd
}
