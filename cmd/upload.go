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
	"fmt"
	"io/ioutil"
	"os"

	"github.com/fox-one/mixin-sdk"
	"github.com/spf13/cobra"
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "upload file to mixin storage",
	Run: func(cmd *cobra.Command, args []string) {
		file, ok := getArg(args, 0)
		if !ok {
			cmd.PrintErr("invalid file path")
			return
		}

		f, err := os.Open(file)
		if err != nil {
			cmd.PrintErr(err)
			return
		}
		defer f.Close()

		body, _ := ioutil.ReadAll(f)

		attachment, err := _dapp.CreateAttachment(ctx)
		if err != nil {
			cmd.PrintErr("create attachment failed", err)
			return
		}

		if err := mixin.UploadAttachment(ctx, attachment, body); err != nil {
			cmd.PrintErr("upload file failed", err)
			return
		}

		fmt.Fprintln(cmd.OutOrStdout(), attachment.AttachmentID)
		fmt.Fprintln(cmd.OutOrStdout(), attachment.ViewURL)
	},
}

func init() {
	_dappCommands = append(_dappCommands, uploadCmd)
}
