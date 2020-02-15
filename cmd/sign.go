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
	"time"

	"github.com/spf13/cobra"
)

// signCmd represents the sign command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "sign jwt token",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, ok := getArg(args, 0)
		if !ok {
			return nil
		}

		if path[0] != '/' {
			path = "/" + path
		}

		exp, _ := cmd.Flags().GetDuration("exp")
		cmd.Printf("sign %s with exp duration %s\n\n", path, exp)
		token, err := _dapp.SignToken("GET", path, nil, exp)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(cmd.OutOrStdout(), token)
		return err
	},
}

func init() {
	_dappCommands = append(_dappCommands, signCmd)
	signCmd.Flags().DurationP("exp", "e", time.Hour, "jwt exp")
}
