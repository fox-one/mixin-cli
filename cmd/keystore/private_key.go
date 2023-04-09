package keystore

import (
	"encoding/json"
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/fox-one/mixin-cli/pkg/column"
	"github.com/fox-one/mixin-cli/pkg/jq"
	"github.com/fox-one/mixin-cli/session"
	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"
)

func NewCmdUpdatePrivateKey() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "private-key <new-private-key-hex>",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			query := args[0]
			uri := fmt.Sprintf("/network/assets/search/%s", query)
			fields := []string{"asset_id", "symbol", "name", "chain_id", "price_usd"}

			parser := jq.ParseObjects
			if uuid.FromStringOrNil(query) != uuid.Nil {
				uri = fmt.Sprintf("/network/assets/%s", query)
				parser = jq.ParseObject
				fields = append(fields, "icon_url")
			}

			for _, field := range args[1:] {
				if !govalidator.IsIn(field, fields...) {
					fields = append(fields, field)
				}
			}

			var body json.RawMessage
			if err := client.Get(ctx, uri, nil, &body); err != nil {
				return err
			}

			lines, err := parser(body, fields...)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), column.Print(lines))
			return nil
		},
	}

	return cmd
}
