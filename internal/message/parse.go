package message

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
)

const (
	plainTextType = "text"
	postType      = "post"
)

func ParseMessageDate(text string) (*mixin.MessageRequest, error) {
	typ := plainTextType

	if items := strings.SplitN(text, ":", 2); len(items) > 1 {
		switch items[0] {
		case plainTextType, postType:
			typ = items[0]
			text = items[1]
		}
	}

	switch typ {
	case plainTextType:
		return parsePlainTextMessage(text)
	case postType:
		return parsePostMessage(text)
	default:
		return nil, errors.New("invalid message type")
	}
}

func parsePlainTextMessage(text string) (*mixin.MessageRequest, error) {
	req := &mixin.MessageRequest{
		Category: mixin.MessageCategoryPlainText,
		Data:     base64.StdEncoding.EncodeToString([]byte(text)),
	}

	return req, nil
}

func parsePostMessage(text string) (*mixin.MessageRequest, error) {
	req := &mixin.MessageRequest{
		Category: "PLAIN_POST",
		Data:     base64.StdEncoding.EncodeToString([]byte(text)),
	}

	return req, nil
}
