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
	"encoding/base64"
	"unicode/utf8"

	"github.com/asaskevich/govalidator"
	"github.com/fox-one/mixin-cli/internal/message"
	"github.com/fox-one/pkg/uuid"
	"github.com/spf13/cobra"
)

var chatCmdFlags struct {
	TraceID   string `valid:"uuid"`
	Recipient string `valid:"uuid,required"`
	Message   string `valid:"required"`
}

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "send a message to somebody",
	Run: func(cmd *cobra.Command, args []string) {
		if chatCmdFlags.TraceID == "" {
			chatCmdFlags.TraceID = uuid.New()
		}

		if _, err := govalidator.ValidateStruct(chatCmdFlags); err != nil {
			cmd.PrintErrln(err)
			return
		}

		conversation, err := _dapp.CreateContactConversation(ctx, chatCmdFlags.Recipient)
		if err != nil {
			cmd.PrintErrln(err)
			return
		}

		if b, err := base64.StdEncoding.DecodeString(chatCmdFlags.Message); err == nil && utf8.Valid(b) {
			chatCmdFlags.Message = string(b)
		}

		req, err := message.ParseMessageDate(chatCmdFlags.Message)
		if err != nil {
			cmd.PrintErrln(err)
			return
		}

		req.MessageID = chatCmdFlags.TraceID
		req.ConversationID = conversation.ConversationID
		req.RecipientID = chatCmdFlags.Recipient

		if err := _dapp.SendMessage(ctx, req); err != nil {
			cmd.PrintErrln(err)
		}
	},
}

func init() {
	_dappCommands = append(_dappCommands, chatCmd)
	chatCmd.Flags().StringVarP(&chatCmdFlags.TraceID, "trace", "t", "", "trace id")
	chatCmd.Flags().StringVarP(&chatCmdFlags.Recipient, "recipient", "r", "", "recipient mixin user id")
	chatCmd.Flags().StringVarP(&chatCmdFlags.Message, "msg", "m", "", "message content")
}
