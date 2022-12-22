package list

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/fox-one/mixin-cli/pkg/column"
	"github.com/fox-one/mixin-cli/pkg/jq"
	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
)

func NewCmdList() *cobra.Command {
	var opt struct {
		input mixin.TransferInput
	}

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

			var assets []*mixin.Asset
			if len(opt.input.OpponentMultisig.Receivers) > 0 && opt.input.OpponentMultisig.Threshold > 0 {
				assets, err = readMultisignAssets(ctx, client, opt.input)
			} else {
				assets, err = client.ReadAssets(ctx)
			}
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

	cmd.Flags().StringSliceVar(&opt.input.OpponentMultisig.Receivers, "receivers", nil, "multisig receivers")
	cmd.Flags().Uint8Var(&opt.input.OpponentMultisig.Threshold, "threshold", 0, "multisig threshold")

	return cmd
}

func readMultisignAssets(ctx context.Context, client *mixin.Client, input mixin.TransferInput) ([]*mixin.Asset, error) {
	assets, err := mixin.ReadMultisigAssets(ctx)
	if err != nil {
		return nil, err
	}
	assetMap := map[string]*mixin.Asset{}
	for _, asset := range assets {
		assetMap[asset.AssetID] = asset
	}

	result := []*mixin.Asset{}
	resultMap := map[string]*mixin.Asset{}
	offset := time.Time{}
	limit := 500
	for {
		outputs, err := client.ListMultisigOutputs(ctx, mixin.ListMultisigOutputsOption{
			Members:        input.OpponentMultisig.Receivers,
			Threshold:      input.OpponentMultisig.Threshold,
			Offset:         offset,
			Limit:          limit,
			OrderByCreated: true,
			State:          mixin.UTXOStateUnspent,
		})
		if err != nil {
			return nil, err
		}
		if len(outputs) == 0 || offset.Equal(outputs[len(outputs)-1].CreatedAt) {
			break
		}

		noMore := len(outputs) < limit
		if outputs[0].CreatedAt.Equal(offset) {
			outputs = outputs[1:]
		}

		for _, output := range outputs {
			a := resultMap[output.AssetID]
			if a == nil {
				a = assetMap[output.AssetID]
				if a == nil {
					a = &mixin.Asset{
						AssetID: output.AssetID,
						Symbol:  "-",
						Name:    "-",
					}
				}
				a.Balance = decimal.Zero
				result = append(result, a)
				resultMap[output.AssetID] = a
			}
			a.Balance = a.Balance.Add(output.Amount)
		}
		if noMore {
			break
		}
		offset = outputs[len(outputs)-1].CreatedAt
	}
	return result, nil
}
