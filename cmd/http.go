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
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "do a http request with GET method",
	Run: func(cmd *cobra.Command, args []string) {
		path, ok := getArg(args, 0)
		if !ok {
			cmd.PrintErr("invalid uri")
			return
		}

		u, err := url.Parse(path)
		if err != nil {
			cmd.PrintErr(err)
			return
		}

		query := u.Query()
		for _, arg := range args[1:] {
			if fields := strings.SplitN(arg, "=", 2); len(fields) == 2 {
				query.Add(fields[0], fields[1])
			}
		}
		u.RawQuery = query.Encode()

		var resp json.RawMessage
		if err := _dapp.Get(ctx, u.String(), nil, &resp); err != nil {
			cmd.PrintErr(err)
			return
		}

		ident, _ := json.MarshalIndent(resp, "", "   ")
		fmt.Fprintln(cmd.OutOrStdout(), string(ident))
	},
}

var postCmd = &cobra.Command{
	Use:   "post",
	Short: "do a http request with POST method",
	Run: func(cmd *cobra.Command, args []string) {
		path, ok := getArg(args, 0)
		if !ok {
			cmd.PrintErr("invalid uri")
			return
		}

		u, err := url.Parse(path)
		if err != nil {
			cmd.PrintErr(err)
			return
		}

		body := map[string]interface{}{}
		for _, arg := range args[1:] {
			if fields := strings.SplitN(arg, "=", 2); len(fields) == 2 {
				body[fields[0]] = fields[1]
			}
		}

		if withPin, _ := cmd.Flags().GetBool("pin"); withPin {
			var pin string
			if _dapp.Pin == "" {
				pin, _ = promptPin()
			} else if conformPay() {
				pin = _dapp.Pin
			}

			if pin == "" {
				return
			}

			encryptPin := _dapp.EncryptPin(pin)

			body["pin"] = encryptPin
		}

		var resp json.RawMessage
		if err := _dapp.Post(ctx, u.String(), body, &resp); err != nil {
			cmd.PrintErr(err)
			return
		}

		ident, _ := json.MarshalIndent(resp, "", "   ")
		fmt.Fprintln(cmd.OutOrStdout(), string(ident))
	},
}

func init() {
	_dappCommands = append(_dappCommands, getCmd)
	_dappCommands = append(_dappCommands, postCmd)
	postCmd.Flags().Bool("pin", false, "post with pin")
}
