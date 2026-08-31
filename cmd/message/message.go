package message

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/spf13/cobra"
)

func NewCmdMessage() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "send messages",
	}

	cmd.AddCommand(NewCmdText())
	return cmd
}

func NewCmdText() *cobra.Command {
	var opt struct {
		text     string
		fromFile string
	}

	cmd := &cobra.Command{
		Use:   "text <user_id>",
		Short: "send a PLAIN_TEXT message to a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)
			client, err := s.GetClient()
			if err != nil {
				return err
			}

			content, err := resolveTextContent(opt.text, opt.fromFile)
			if err != nil {
				return err
			}

			user, err := client.ReadUser(ctx, args[0])
			if err != nil {
				return fmt.Errorf("read user %q failed: %w", args[0], err)
			}

			if _, err := client.CreateContactConversation(ctx, user.UserID); err != nil {
				return fmt.Errorf("create conversation failed: %w", err)
			}

			req := &mixin.MessageRequest{
				ConversationID: mixin.UniqueConversationID(client.ClientID, user.UserID),
				RecipientID:    user.UserID,
				MessageID:      mixin.RandomTraceID(),
				Category:       mixin.MessageCategoryPlainText,
				Data:           base64.StdEncoding.EncodeToString([]byte(content)),
			}
			if err := client.SendMessage(ctx, req); err != nil {
				return fmt.Errorf("send message failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), req.MessageID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opt.text, "text", "t", "", "message text")
	cmd.Flags().StringVar(&opt.fromFile, "from-file", "", "read message text from file")

	return cmd
}

func resolveTextContent(text, fromFile string) (string, error) {
	switch {
	case text != "" && fromFile != "":
		return "", errors.New("only one of --text or --from-file can be set")
	case fromFile != "":
		b, err := os.ReadFile(fromFile)
		if err != nil {
			return "", fmt.Errorf("read file %s failed: %w", fromFile, err)
		}
		if len(b) == 0 {
			return "", errors.New("message text is empty")
		}
		return string(b), nil
	case text != "":
		return text, nil
	default:
		return "", errors.New("--text or --from-file is required")
	}
}
