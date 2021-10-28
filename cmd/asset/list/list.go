package list

import (
	"encoding/json"
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/fox-one/mixin-cli/pkg/column"
	"github.com/fox-one/mixin-cli/pkg/jq"
	"github.com/fox-one/mixin-cli/session"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdList() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list assets",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			assets, err := client.ReadAssets(ctx)
			if err != nil {
				return err
			}

			// filter assets with positive balance
			var (
				idx        int
				totalValue decimal.Decimal
			)

			for _, asset := range assets {
				if !asset.Balance.IsPositive() {
					continue
				}

				totalValue = totalValue.Add(asset.PriceUSD.Mul(asset.Balance))
				assets[idx] = asset
				idx++
			}
			assets = assets[:idx]

			data, _ := json.Marshal(assets)
			fields := []string{"asset_id", "symbol", "name", "balance"}
			for _, arg := range args {
				if !govalidator.IsIn(arg, fields...) {
					fields = append(fields, arg)
				}
			}

			lines, err := jq.ParseObjects(data, fields...)
			if err != nil {
				return err
			}

			fmt.Println(column.Print(lines))
			fmt.Println("Total USD Value:", totalValue)
			return nil
		},
	}

	return cmd
}
