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
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/go-yaml/yaml"
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "show keystore content",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.Open(_keyStoreFile)
		if err != nil {
			cmd.PrintErr(err)
			return
		}
		defer f.Close()

		var r io.Reader = f

		if outputYaml, _ := cmd.Flags().GetBool("yaml"); outputYaml {
			var i interface{}
			if err := json.NewDecoder(r).Decode(&i); err != nil {
				cmd.PrintErr(err)
				return
			}

			data, err := yaml.Marshal(i)
			if err != nil {
				cmd.PrintErr(err)
				return
			}

			r = bytes.NewBuffer(data)
		}

		if _, err := io.Copy(cmd.OutOrStdout(), r); err != nil {
			cmd.PrintErr(err)
			return
		}
	},
}

func init() {
	_dappCommands = append(_dappCommands, showCmd)
	showCmd.Flags().Bool("yaml", false, "yaml format")
}
