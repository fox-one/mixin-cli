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
	"errors"
	"unicode/utf8"

	"github.com/fox-one/mixin-sdk"
	"github.com/fox-one/pkg/number"
	"github.com/fox-one/pkg/text/columnize"
	"github.com/fox-one/pkg/uuid"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// payCmd represents the pay command
var payCmd = &cobra.Command{
	Use:   "pay",
	Short: "transfer assets to other users",
	Long:  "pay {opponent_id} {asset_id} {amount} {memo}",
	RunE: func(cmd *cobra.Command, args []string) error {
		opponent, ok := getArg(args, 0)
		if !ok || !uuid.IsUUID(opponent) {
			return errors.New("invalid opponent id")
		}

		profile, err := _dapp.FetchUser(ctx, opponent)
		if err != nil {
			cmd.PrintErrf("fetch opponent profile failed: %w", err)
		}
		opponentName := profile.FullName
		if opponentName == "" {
			opponentName = "anonymous"
		}

		assetID, ok := getArg(args, 1)
		if !ok || !uuid.IsUUID(assetID) {
			return errors.New("invalid asset id")
		}

		asset, err := _dapp.ReadAsset(ctx, assetID)
		if err != nil {
			cmd.PrintErrf("read asset failed: %w", err)
		}

		amount, _ := getArg(args, 2)
		if !number.Decimal(amount).IsPositive() {
			return errors.New("amount must be positive")
		}

		memo, _ := getArg(args, 3)
		if utf8.RuneCountInString(memo) > 140 {
			return errors.New("memo must have less than 140 characters")
		}

		cmd.Printf("pay %s %s to %s\n", amount, asset.Symbol, opponentName)

		var pin string
		if _dapp.Pin == "" {
			pin, _ = promptPin()
		} else if conformPay() {
			pin = _dapp.Pin
		}

		if pin == "" {
			return nil
		}

		snapshot, err := _dapp.Transfer(ctx, &mixin.TransferInput{
			AssetID:    assetID,
			OpponentID: opponent,
			Amount:     amount,
			TraceID:    uuid.New(),
			Memo:       memo,
		}, pin)
		if err != nil {
			return err
		}

		form := columnize.Form{}
		form.Append("🎉 payment successful")
		form.Append("Snapshot ID", snapshot.SnapshotID)
		form.Append("Trace ID", snapshot.TraceID)
		return form.Fprint(cmd.OutOrStdout())
	},
}

func init() {
	_dappCommands = append(_dappCommands, payCmd)
}

func promptPin() (string, error) {
	prompt := promptui.Prompt{
		Label: "Pin",
		Mask:  '*',
		Validate: func(input string) error {
			if len(input) != 6 {
				return errors.New("pin must have 6 characters exactly")
			}

			return nil
		},
	}

	return prompt.Run()
}

func conformPay() bool {
	prompt := promptui.Prompt{
		Label:     "Continue",
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if err != nil {
		return false
	}

	switch result {
	case "y", "Y":
		return true
	default:
		return false
	}
}
