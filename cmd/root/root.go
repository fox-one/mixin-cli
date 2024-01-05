package root

import (
	"fmt"
	"net/url"
	"os"

	"github.com/andrew-d/go-termutil"
	"github.com/fox-one/mixin-cli/v2/cmd/asset"
	"github.com/fox-one/mixin-cli/v2/cmd/http"
	"github.com/fox-one/mixin-cli/v2/cmd/keystore"
	"github.com/fox-one/mixin-cli/v2/cmd/ownership"
	"github.com/fox-one/mixin-cli/v2/cmd/pay"
	"github.com/fox-one/mixin-cli/v2/cmd/safe"
	"github.com/fox-one/mixin-cli/v2/cmd/sign"
	"github.com/fox-one/mixin-cli/v2/cmd/tr"
	"github.com/fox-one/mixin-cli/v2/cmd/transfer"
	"github.com/fox-one/mixin-cli/v2/cmd/upload"
	"github.com/fox-one/mixin-cli/v2/cmd/user"
	"github.com/fox-one/mixin-cli/v2/cmd/withdraw"
	"github.com/fox-one/mixin-cli/v2/cmdutil"
	"github.com/fox-one/mixin-cli/v2/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdRoot(version string) *cobra.Command {
	var opt struct {
		host         string
		KeystoreFile string
		accessToken  string
		Pin          string
		Stdin        bool
	}

	cmd := &cobra.Command{
		Use:           "mixin-cli <command> <subcommand> [flags]",
		Short:         "Mixin CLI",
		Long:          `Work seamlessly with Mixin from the command line.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			s := session.From(cmd.Context())

			v := viper.New()
			v.SetConfigType("json")
			v.SetConfigType("yaml")

			if opt.KeystoreFile != "" {
				f, err := os.Open(opt.KeystoreFile)
				if err != nil {
					return fmt.Errorf("open keystore file %s failed: %w", opt.KeystoreFile, err)
				}

				defer f.Close()
				_ = v.ReadConfig(f)
			} else if opt.Stdin && isStdinAvailable() {
				_ = v.ReadConfig(cmd.InOrStdin())
			}

			if values := v.AllSettings(); len(values) > 0 {
				b, _ := jsoniter.Marshal(values)
				store, err := cmdutil.DecodeKeystore(b)
				if err != nil {
					return fmt.Errorf("decode keystore failed: %w", err)
				}

				s.WithKeystore(store.Keystore)

				pin := store.Pin
				if opt.Pin != "" {
					pin = opt.Pin
				}
				if store.SpendKey != "" || len(pin) > 6 {
					client, err := mixin.NewFromKeystore(store.Keystore)
					if err != nil {
						return fmt.Errorf("new client failed: %w", err)
					}
					user, err := client.UserMe(cmd.Context())
					if err != nil {
						return fmt.Errorf("user me failed: %w", err)
					}

					if store.SpendKey != "" && user.SpendPublicKey != "" {
						spendKey, err := mixinnet.ParseKeyWithPub(store.SpendKey, user.SpendPublicKey)
						if err != nil {
							return fmt.Errorf("parse spend key failed: %w", err)
						}
						s.WithSpendKey(&spendKey)
					}

					if len(pin) > 6 && user.TipKeyBase64 != "" {
						pinKey, err := mixinnet.ParseKeyWithPub(pin, user.TipKeyBase64)
						if err != nil {
							return fmt.Errorf("parse pin key failed: %w", err)
						}
						s.WithPin(pinKey.String())
					}
				}
			}

			if opt.accessToken != "" {
				s.WithAccessToken(opt.accessToken)
			}

			if cmd.Flags().Changed("host") {
				u, err := url.Parse(opt.host)
				if err != nil {
					return err
				}

				if u.Scheme == "" {
					u.Scheme = "https"
				}

				mixin.UseApiHost(u.String())
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&opt.host, "host", mixin.DefaultApiHost, "custom api host")
	cmd.PersistentFlags().StringVarP(&opt.KeystoreFile, "file", "f", "", "keystore file path (default is $HOME/.mixin-cli/keystore.json)")
	cmd.PersistentFlags().StringVar(&opt.accessToken, "token", "", "custom access token")
	cmd.PersistentFlags().StringVar(&opt.Pin, "pin", "", "raw pin")
	cmd.PersistentFlags().BoolVar(&opt.Stdin, "stdin", false, "read keystore from standard input")

	cmd.AddCommand(sign.NewCmdSign())
	cmd.AddCommand(http.NewCmdHttp())
	cmd.AddCommand(user.NewCmdUser())
	cmd.AddCommand(upload.NewCmdUpload())
	cmd.AddCommand(pay.NewCmdPay())
	cmd.AddCommand(transfer.NewCmdTransfer())
	cmd.AddCommand(withdraw.NewCmdWithdraw())
	cmd.AddCommand(asset.NewCmdAsset())
	cmd.AddCommand(keystore.NewCmdKeystore())
	cmd.AddCommand(ownership.NewCmdOwnership())
	cmd.AddCommand(safe.NewCmdSafe())
	tr.Bind(cmd)

	return cmd
}

func isStdinAvailable() bool {
	return !termutil.Isatty(os.Stdin.Fd())
}
