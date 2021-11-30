package column

import (
	"github.com/ryanuber/columnize"
)

func Print(lines []string) string {
	cfg := columnize.DefaultConfig()
	cfg.Delim = ","
	cfg.Glue = "  "
	cfg.Prefix = ""
	cfg.Empty = "NULL"

	return columnize.Format(lines, cfg)
}
