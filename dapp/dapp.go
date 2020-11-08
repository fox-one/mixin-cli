package dapp

import (
	"github.com/fox-one/mixin-sdk-go"
)

type Dapp struct {
	*mixin.Client
	Pin  string
	Name string
}

func New(store *KeyStore) (*Dapp, error) {
	c, err := mixin.NewFromKeystore(
		&mixin.Keystore{
			ClientID:   store.UserID,
			SessionID:  store.SessionID,
			PrivateKey: store.PrivateKey,
			PinToken:   store.PinToken,
			Scope:      "FULL",
		},
	)
	if err != nil {
		return nil, err
	}
	return &Dapp{
		Client: c,
		Pin:    store.Pin,
	}, nil
}
