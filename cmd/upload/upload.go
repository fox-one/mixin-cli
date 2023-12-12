package upload

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/spf13/cobra"
)

func NewCmdUpload() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <file>",
		Short: "upload file to mixin storage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)
			client, err := s.GetClient()
			if err != nil {
				return err
			}

			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()

			body, _ := ioutil.ReadAll(f)

			attachment, err := client.CreateAttachment(ctx)
			if err != nil {
				return err
			}

			if err := mixin.UploadAttachment(ctx, attachment, body); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), attachment.AttachmentID)
			fmt.Fprintln(cmd.OutOrStdout(), attachment.ViewURL)
			return nil
		},
	}

	return cmd
}
