package safe

import (
	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdAssets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "list assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			utxos, err := listUnspentOutputs(ctx, client)
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

			cmd.Println("asset", "count", "balance")
			for a, utxos := range utxos {
				asset, ok := assetM[a]
				if !ok {
					a, err := client.SafeReadAsset(ctx, a)
					if err != nil {
						cmd.Println("read asset failed:", err)
						return err
					}
					asset = a
				}

				count := len(utxos)
				balance := decimal.Zero
				for _, utxo := range utxos {
					balance = balance.Add(utxo.Amount)
				}
				cmd.Println(asset.Symbol, asset.AssetID, count, balance)
			}
			return nil
		},
	}

	return cmd
}
