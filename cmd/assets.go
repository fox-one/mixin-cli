/*
Copyright © 2020 yiplee <guoyinl@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fox-one/mixin-sdk"
	"github.com/fox-one/pkg/text/columnize"
	"github.com/fox-one/pkg/uuid"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

// assetsCmd represents the assets command
var assetsCmd = &cobra.Command{
	Use:     "assets",
	Aliases: []string{"asset"},
	Short:   "show dapp assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		assets, err := _dapp.ReadAssets(ctx)
		if err != nil {
			return err
		}

		// sort assets by usd value desc
		sort.Slice(assets, func(i, j int) bool {
			a1, a2 := assets[i], assets[j]
			v1, v2 := assetUSDValue(a1), assetUSDValue(a2)
			if v1.Equal(v2) {
				return a1.PriceUsd.GreaterThan(a2.PriceUsd)
			}

			return v1.GreaterThan(v2)
		})

		if arg, ok := getArg(args, 0); ok {
			var argType string
			if uuid.IsUUID(arg) {
				argType = "id"
			} else {
				argType = "symbol"
				arg = strings.ToUpper(arg)
			}

			for _, asset := range assets {
				switch arg {
				case asset.Symbol, asset.AssetID:
					form := columnizeAsset(asset)
					return form.Fprint(cmd.OutOrStdout())
				}
			}

			if argType == "id" {
				asset, err := _dapp.ReadAsset(ctx, arg)
				if err != nil {
					return err
				}

				form := columnizeAsset(asset)
				return form.Fprint(cmd.OutOrStdout())
			}

			return fmt.Errorf("asset not found with %s %s", argType, arg)
		}

		var totalValue decimal.Decimal

		var (
			idx    = 0
			all, _ = cmd.Flags().GetBool("all")
		)
		for _, asset := range assets {
			if asset.Balance.IsZero() && !all {
				continue
			}

			totalValue = totalValue.Add(assetUSDValue(asset))
			assets[idx] = asset
			idx++
		}

		assets = assets[:idx]

		if len(assets) > 0 {
			form := columnizeAssets(assets)
			_ = form.Fprint(cmd.OutOrStdout())
		}

		cmd.Println("Total Values:", totalValue.String(), "USD")
		return nil
	},
}

func init() {
	_dappCommands = append(_dappCommands, assetsCmd)
	assetsCmd.Flags().BoolP("all", "a", false, "show all assets")
}

func assetUSDValue(asset *mixin.Asset) decimal.Decimal {
	return asset.Balance.Mul(asset.PriceUsd)
}

func columnizeAsset(asset *mixin.Asset) (form columnize.Form) {
	form.Append("Asset ID", asset.AssetID)
	form.Append("Symbol", asset.Symbol)
	form.Append("Name", asset.Name)
	form.Append("Balance", asset.Balance.String())
	form.Append("Price(USD)", asset.PriceUsd.String())
	form.Append("Change", asset.ChangeUsd.Shift(2).Truncate(2).String()+"%")
	form.Append("Value(USD)", assetUSDValue(asset).StringFixed(2))
	form.Append("Destination", asset.Destination)
	form.Append("Tag", asset.Tag)
	form.Append("Logo", asset.IconURL)

	return
}

func columnizeAssets(assets []*mixin.Asset) (form columnize.Form) {
	form.Append("Asset ID", "Symbol", "Name", "Price(USD)", "Change", "Balance", "Value(USD)")
	for _, asset := range assets {
		form.Append(
			asset.AssetID,
			asset.Symbol,
			asset.Name,
			asset.PriceUsd.String(),
			asset.ChangeUsd.Shift(2).Truncate(2).String()+"%",
			asset.Balance.String(),
			assetUSDValue(asset).StringFixed(2),
		)
	}

	return
}
