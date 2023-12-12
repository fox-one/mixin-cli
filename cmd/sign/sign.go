package sign

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/nojima/httpie-go/exchange"
	"github.com/nojima/httpie-go/input"
	"github.com/spf13/cobra"
)

func NewCmdSign() *cobra.Command {
	var opt struct {
		exp         time.Duration
		raw         string
		scope       string
		dumpRequest bool
	}

	cmd := &cobra.Command{
		Use:   "sign",
		Args:  cobra.MinimumNArgs(1),
		Short: "sign token with custom exp & scope",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := session.From(cmd.Context())

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

			if opt.dumpRequest {
				b, err := httputil.DumpRequest(req, true)
				if err != nil {
					return err
				}

				cmd.Println(string(b), "\n")
			}

			sig := mixin.SignRequest(req)
			store, err := s.GetKeystore()
			if err != nil {
				return err
			}

			if opt.scope != "" {
				store.Scope = opt.scope
			}

			auth, err := mixin.AuthFromKeystore(store)
			if err != nil {
				return err
			}

			requestID := mixin.RandomTraceID()
			uri := req.URL.Path
			if q := req.URL.RawQuery; q != "" {
				uri += "?" + q
			}

			cmd.Printf("sign %s %s with request id %s & exp %s\n\n", req.Method, uri, requestID, opt.exp)
			token := auth.SignToken(sig, requestID, opt.exp)
			_, err = fmt.Fprint(cmd.OutOrStdout(), token)
			return err
		},
	}

	cmd.Flags().DurationVar(&opt.exp, "exp", time.Minute, "token expire duration")
	cmd.Flags().StringVar(&opt.raw, "raw", "", "raw json body")
	cmd.Flags().BoolVar(&opt.dumpRequest, "dump", false, "dump request information")
	cmd.Flags().StringVar(&opt.scope, "scope", "FULL", "jwt token scope")

	return cmd
}
