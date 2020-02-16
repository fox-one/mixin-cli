/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

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
	"github.com/spf13/cobra"
)

// pinCmd represents the pin command
var pinCmd = &cobra.Command{
	Use:   "pin",
	Short: "update pin",
	Long:  "pin {new pin}",
	RunE: func(cmd *cobra.Command, args []string) error {
		newPin, _ := getArg(args, 0)
		if err := validatePin(newPin); err != nil {
			return err
		}

		cmd.Printf("Update pin to %s\n", newPin)

		var pin string
		if _dapp.Pin == "" {
			pin, _ = promptPin()
		} else if conformPay() {
			pin = _dapp.Pin
		}

		if pin == "" {
			return nil
		}

		if err := _dapp.ModifyPIN(ctx, pin, newPin); err != nil {
			return err
		}

		cmd.Println("🎉 update pin successful")
		if _dapp.Pin != "" && _dapp.Pin != newPin {
			cmd.Println("⏰ remember to update the keystore file with new pin")
			_dapp.Pin = newPin
		}
		return nil
	},
}

func init() {
	_dappCommands = append(_dappCommands, pinCmd)
}
