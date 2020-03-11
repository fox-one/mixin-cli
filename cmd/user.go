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
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/fox-one/mixin-cli/dapp"
	"github.com/fox-one/pkg/encrypt"
	"github.com/fox-one/pkg/number"
	"github.com/spf13/cobra"
)

// userCmd represents the user command
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "create a new user",
	Run: func(cmd *cobra.Command, args []string) {
		name, ok := getArg(args, 0)
		if !ok {
			cmd.PrintErrln("user name must assigned")
			return
		}

		key := encrypt.RSA.New()
		user, err := _dapp.CreateUser(ctx, key, name)
		if err != nil {
			cmd.PrintErrln("create user", err)
			return
		}

		sessionKey := pem.EncodeToMemory(&pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   encrypt.RSA.Marshal(key),
		})

		store := dapp.KeyStore{
			UserID:     user.UserID,
			SessionID:  user.SessionID,
			PrivateKey: string(sessionKey),
			PinToken:   user.PINToken,
			Pin:        "",
		}

		pin, _ := cmd.Flags().GetString("pin")
		if pin == "" {
			pin = number.RandomPin()
		}

		if err := user.ModifyPIN(ctx, "", pin); err != nil {
			cmd.PrintErrln("update pin", err)
			return
		}

		store.Pin = pin
		out, _ := json.MarshalIndent(store, "", "  ")

		if outPath, _ := cmd.Flags().GetString("out"); outPath != "" {
			filename := fmt.Sprintf("%s.json", name)
			outPath = path.Join(outPath, filename)
			if err := ioutil.WriteFile(outPath, out, 0644); err != nil {
				cmd.PrintErrln("write file", err)
			}

			return
		}

		_, _ = cmd.OutOrStdout().Write(out)
		cmd.Println()
	},
}

func init() {
	_dappCommands = append(_dappCommands, userCmd)
	userCmd.Flags().String("pin", "", "pin")
	userCmd.Flags().StringP("out", "o", "", "output file path")
}
