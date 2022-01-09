package tr

import (
	"crypto/md5"
	"io"
	"strings"

	"github.com/gofrs/uuid"
)

const ethAssetID = "43d61dcd-e413-450d-80b8-101d5e903357"

func addressToAssetID(contractAddr string) string {
	h := md5.New()
	_, _ = io.WriteString(h, ethAssetID)
	_, _ = io.WriteString(h, strings.ToLower(contractAddr))
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return uuid.FromBytesOrNil(sum).String()
}
