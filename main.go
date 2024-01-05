/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
package main

import (
	"context"
	"encoding/hex"
	"os"

	"github.com/fox-one/mixin-cli/v2/cmd/root"
	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	"github.com/spf13/cobra"
)

var (
	version = "2.1.3"
)

func main() {
	ctx := context.Background()
	s := &session.Session{Version: version}
	ctx = session.With(ctx, s)

	expandedArgs := []string{}
	if len(os.Args) > 0 {
		expandedArgs = os.Args[1:]
	}

	rootCmd := root.NewCmdRoot(version)

	if len(expandedArgs) > 0 && !hasCommand(rootCmd, expandedArgs) {
		name := expandedArgs[0]
		if b, err := cmdutil.LookupAndLoadKeystore(name); err == nil {
			if store, err := cmdutil.DecodeKeystore(b); err == nil {
				s.WithKeystore(store.Keystore)
				pin := store.Pin
				if len(pin) > 6 || store.SpendKey != "" {
					client, err := mixin.NewFromKeystore(store.Keystore)
					if err != nil {
						rootCmd.PrintErrln("new client failed:", err)
						os.Exit(1)
					}

					user, err := client.UserMe(ctx)
					if err != nil {
						rootCmd.PrintErrln("user me failed:", err)
						os.Exit(1)
					}

					if len(pin) > 6 && user.TipKeyBase64 != "" {
						tipPub := user.TipKeyBase64
						if tipPubBts, err := cmdutil.DecodeBase64(tipPub); err == nil {
							tipPub = hex.EncodeToString(tipPubBts)
						}
						pinKey, err := mixinnet.ParseKeyWithPub(pin, tipPub)
						if err != nil {
							rootCmd.PrintErrln("parse pin failed:", err)
							os.Exit(1)
						}
						pin = pinKey.String()
					}
					if store.SpendKey != "" && user.SpendPublicKey != "" {
						spendKey, err := mixinnet.ParseKeyWithPub(store.SpendKey, user.SpendPublicKey)
						if err != nil {
							rootCmd.PrintErrln("parse spend key failed:", err)
							os.Exit(1)
						}
						s.WithSpendKey(&spendKey)
					}
				}
				s.WithPin(pin)

				expandedArgs = expandedArgs[1:]
			}
		}
	}

	rootCmd.SetArgs(expandedArgs)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		rootCmd.PrintErrln("execute failed:", err)
		os.Exit(1)
	}
}

func hasCommand(rootCmd *cobra.Command, args []string) bool {
	c, _, err := rootCmd.Traverse(args)
	return err == nil && c != rootCmd
}
