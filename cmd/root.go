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
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/fox-one/mixin-cli/dapp"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

var (
	ctx                  = context.Background()
	_dapp                *dapp.Dapp
	_dappCommands        []*cobra.Command
	_interactiveCommands []*cobra.Command

	errQuit = errors.New("quit")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:                "mixin-cli",
	Short:              "manager your mixin dapps",
	Version:            "1.0.0 by Fox.ONE",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _dapp == nil {
			name, ok := getArg(args, 0)

			cmd.DisableFlagParsing = false
			if strings.HasPrefix(name, "-") {
				return cmd.Execute()
			}

			if ok {
				args = args[1:]
			}

			app, err := initDapp(name)
			if err != nil {
				cmd.PrintErr(err)
				return nil
			}

			_dapp = app
			cmd.AddCommand(_dappCommands...)
			cmd.SetArgs(args)
			return cmd.Execute()
		}

		if len(args) > 0 {
			return cmd.Usage()
		}

		// promote for action
		// enable interactive mode
		cmd.SilenceUsage = true
		cmd.AddCommand(_interactiveCommands...)

		for {
			if err := promptDappCommand(cmd); err == nil {
				continue
			} else if err == errQuit {
				break
			} else {
				cmd.PrintErr(err)
			}
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initDapp(name string) (*dapp.Dapp, error) {
	file, err := selectKeyStoreFile(name)
	if err != nil {
		return nil, err
	}

	store, err := dapp.LoadKeyStoreFromFile(file)
	if err != nil {
		return nil, err
	}

	app, err := dapp.New(store)
	if err != nil {
		return nil, err
	}

	if profile, err := app.FetchProfile(ctx); err == nil {
		app.Name = profile.FullName
	}

	return app, nil
}

func selectKeyStoreFile(name string) (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	root := path.Join(home, ".mixin-cli")
	files := dapp.SearchKeyStoreFiles(root, name)
	switch len(files) {
	case 0:
		return "", fmt.Errorf("no keystore file found in %s", root)
	case 1:
		return files[0], nil
	}

	prompt := promptui.Select{
		Label: "select a keystore file",
		Items: files,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return files[idx], nil
}

func promptDappCommand(cmd *cobra.Command) error {
	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | blue }} ",
		Invalid: "{{ . | blue }} ",
		Success: "{{ . | blue }} ",
	}

	label := "❯"
	if _dapp.Name != "" {
		label = _dapp.Name + label
	}

	prompt := promptui.Prompt{
		Label:     label,
		Validate:  nil,
		Templates: templates,
		Pointer:   promptui.PipeCursor,
	}

	result, err := prompt.Run()
	if err != nil {
		return err
	}

	args := strings.Fields(result)
	if len(args) == 0 {
		return nil
	}

	cmd.SetArgs(args)
	return cmd.Execute()
}
