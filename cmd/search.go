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
	"errors"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/fox-one/pkg/uuid"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search mixin",
}

var searchUserCmd = &cobra.Command{
	Use:   "user",
	Short: "search user by identity number or user id",
	RunE: func(cmd *cobra.Command, args []string) error {
		arg, _ := getArg(args, 0)

		var (
			profile *mixin.User
			err     error
		)

		switch {
		case uuid.IsUUID(arg):
			profile, err = _dapp.ReadUser(ctx, arg)
		case cast.ToInt64(arg) > 0:
			profile, err = _dapp.SearchUser(ctx, arg)
		default:
			err = errors.New("invalid argument")
		}

		if err != nil {
			return err
		}

		form := columnizeProfile(profile)
		return form.Fprint(cmd.OutOrStdout())
	},
}

func init() {
	_dappCommands = append(_dappCommands, searchCmd)
	searchCmd.AddCommand(searchUserCmd)
}
