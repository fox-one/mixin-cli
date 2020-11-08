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
	"github.com/fox-one/mixin-sdk-go"
	"github.com/fox-one/pkg/text/columnize"
	"github.com/spf13/cobra"
)

// meCmd represents the me command
var meCmd = &cobra.Command{
	Use:   "me",
	Short: "show current dapp's profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		me, err := _dapp.UserMe(ctx)
		if err != nil {
			return err
		}

		form := columnizeProfile(me)
		return form.Fprint(cmd.OutOrStdout())
	},
}

func init() {
	_dappCommands = append(_dappCommands, meCmd)
}

func columnizeProfile(p *mixin.User) columnize.Form {
	form := columnize.Form{}
	form.Append("identity", p.IdentityNumber)
	form.Append("fullname", p.FullName)
	form.Append("user_id", p.UserID)
	form.Append("mixin_url", "mixin://users/"+p.UserID)
	form.Append("avatar", p.AvatarURL)
	return form
}
