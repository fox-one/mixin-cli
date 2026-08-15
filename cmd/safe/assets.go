package safe

import (
	"sort"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdAssets() *cobra.Command {
	var opt struct {
		receivers []string
		threshold uint8
	}

	cmd := &cobra.Command{
		Use:   "assets",
		Short: "list safe assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			utxos, err := listUnspentOutputs(ctx, client, opt.receivers, opt.threshold)
			if err != nil {
				return err
			}

			assets, err := client.SafeReadAssets(ctx)
			if err != nil {
				cmd.Println("read assets failed:", err)
				return err
			}

			assetM := map[string]*mixin.SafeAsset{}
			for _, asset := range assets {
				assetM[asset.AssetID] = asset
			}

			assetIDs := make([]string, 0, len(utxos))
			for assetID := range utxos {
				assetIDs = append(assetIDs, assetID)
			}
			sort.Strings(assetIDs)

			cmd.Println("asset", "count", "balance")
			for _, assetID := range assetIDs {
				outputs := utxos[assetID]
				asset, ok := assetM[assetID]
				if !ok {
					a, err := client.SafeReadAsset(ctx, assetID)
					if err != nil {
						cmd.Println("read asset failed:", err)
						return err
					}
					asset = a
				}

				count := len(outputs)
				balance := decimal.Zero
				for _, utxo := range outputs {
					balance = balance.Add(utxo.Amount)
				}
				cmd.Println(asset.Symbol, asset.AssetID, count, balance)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "safe multisig members or one MIX address")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 0, "safe multisig threshold")

	return cmd
}
