package dapp

import (
	"github.com/fox-one/mixin-sdk"
)

type Dapp struct {
	*mixin.User
	Pin  string
	Name string
}

func New(store *KeyStore) (*Dapp, error) {
	user, err := mixin.NewUser(
		store.UserID,
		store.SessionID,
		store.PrivateKey,
		store.PinToken,
	)
	if err != nil {
		return nil, err
	}

	return &Dapp{
		User: user,
		Pin:  store.Pin,
	}, nil
}
