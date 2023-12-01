package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/nojima/httpie-go/exchange"
	"github.com/nojima/httpie-go/input"
	"github.com/spf13/cobra"
)

func NewCmdHttp() *cobra.Command {
	var opt struct {
		raw         string
		dumpRequest bool
	}

	cmd := &cobra.Command{
		Use:   "http",
		Args:  cobra.MinimumNArgs(1),
		Short: "mixin api http client",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)
			client, err := s.GetClient()
			if err != nil {
				return err
			}

			in, err := input.ParseArgs(args, cmd.InOrStdin(), &input.Options{JSON: true})
			if err != nil {
				return err
			}

			host, _ := url.Parse(mixin.GetRestyClient().HostURL)
			in.URL.Host = host.Host
			in.URL.Scheme = host.Scheme

			in.Header.Fields = append(in.Header.Fields, input.Field{
				Name:  "User-Agent",
				Value: "mixin-cli/" + s.Version,
			})

			if opt.raw != "" {
				var raw json.RawMessage
				if err := json.Unmarshal([]byte(opt.raw), &raw); err != nil {
					return fmt.Errorf("json decode raw body failed: %w", err)
				}

				in.Method = http.MethodPost
				in.Body = input.Body{
					BodyType: input.RawBody,
					Raw:      raw,
				}
			}

			req, err := exchange.BuildHTTPRequest(in, &exchange.Options{})
			if err != nil {
				return err
			}

			sig := mixin.SignRequest(req)
			if token := client.Signer.SignToken(sig, mixin.RandomTraceID(), time.Minute); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")

			if opt.dumpRequest {
				b, err := httputil.DumpRequest(req, true)
				if err != nil {
					return err
				}

				cmd.Println(string(b), "\n")
			}

			resp, err := mixin.GetClient().Do(req)
			if err != nil {
				return err
			}

			defer resp.Body.Close()

			var body struct {
				Error *mixin.Error
				Data  json.RawMessage `json:"data"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				return fmt.Errorf("decode json body failed: %w", err)
			}

			if body.Error != nil && body.Error.Code > 0 {
				return body.Error
			}

			data, _ := json.MarshalIndent(body.Data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&opt.raw, "raw", "", "raw json body")
	cmd.Flags().BoolVarP(&opt.dumpRequest, "dump", "v", false, "dump request information")

	return cmd
}
