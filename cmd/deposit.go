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
	"github.com/fox-one/pkg/qrcode"
	"github.com/spf13/cobra"
)

// depositCmd represents the deposit command
var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "show payment qrcode",
	Run: func(cmd *cobra.Command, args []string) {
		url := "mixin://transfer/" + _dapp.UserID
		qrcode.Fprint(cmd.OutOrStdout(), url)
		cmd.Println("scan the qrcode above by fox or mixin messenger")
	},
}

func init() {
	_dappCommands = append(_dappCommands, depositCmd)
}
