package safe

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fox-one/mixin-cli/session"
	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/spf13/cobra"
)

func NewCmdDeposit() *cobra.Command {
	var opt struct {
		receivers []string
		threshold uint8
		chain     string
	}
	cmd := &cobra.Command{
		Use:   "deposit --chain UUID",
		Short: "get deposit address",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			entries, err := client.SafeCreateDepositEntries(ctx, opt.receivers, int(opt.threshold), opt.chain)
			if err != nil {
				return fmt.Errorf("create deposit entries failed: %w", err)
			}
			for _, entry := range entries {
				cmd.Printf("Destination: %s\n", entry.Destination)
				cmd.Printf("Tag: %s\n", entry.Tag)
				cmd.Printf("Members: %v\n", entry.Members)
				cmd.Printf("Threshold: %d\n", entry.Threshold)
				cmd.Printf("EntryID: %s\n", entry.EntryID)
				cmd.Printf("Signature: %s\n", entry.Signature)

				json, err := json.Marshal(entry)
				if err != nil {
					cmd.Println("Error marshaling JSON:", err)
					continue
				}
				cmd.Printf("\nRaw: %s\n", string(json))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opt.chain, "chain", "", "chain id")
	cmd.Flags().StringSliceVar(&opt.receivers, "receivers", nil, "multisig receivers")
	cmd.Flags().Uint8Var(&opt.threshold, "threshold", 1, "multisig threshold")
	return cmd
}

func NewCmdListDeposit() *cobra.Command {
	var opt struct {
		entry  string
		asset  string
		offset string
		limit  uint8
	}
	cmd := &cobra.Command{
		Use:   "pending --entry 'raw json'",
		Short: "get pending deposit",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s := session.From(ctx)

			client, err := s.GetClient()
			if err != nil {
				return err
			}

			var e mixin.SafeDepositEntry
			err = json.Unmarshal([]byte(opt.entry), &e)
			if err != nil {
				return fmt.Errorf("Failed to parse deposit entry raw json: %w", err)
			}

			var t time.Time
			if opt.offset != "" {
				t, err = time.Parse(time.RFC3339Nano, opt.offset)
				if err != nil {
					return fmt.Errorf("Error parsing time: %w", err)
				}
			} else {
				t = time.Time{}
			}

			var a string
			if opt.asset != "" {
				a = opt.asset
			} else {
				a = ""
			}

			deposit, err := client.SafeListDeposits(ctx, &e, a, t, int(opt.limit))
			if err != nil {
				return fmt.Errorf("client.SafeListDeposits() =>: %w", err)
			}
			cmd.Printf("Deposit: %+v\n", deposit)

			json, err := json.Marshal(deposit)
			if err != nil {
				return fmt.Errorf("Error marshaling JSON: %w", err)
			}
			cmd.Printf("\nRaw: %s\n", string(json))
			return nil
		},
	}
	cmd.Flags().StringVar(&opt.entry, "entry", "", "the raw string of entry")
	cmd.Flags().StringVar(&opt.asset, "asset", "", "the uuid of asset")
	cmd.Flags().StringVar(&opt.offset, "offset", "", "offset")
	cmd.Flags().Uint8Var(&opt.limit, "limit", 100, "limit")
	return cmd
}
