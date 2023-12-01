package session

import (
	"errors"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
)

var (
	ErrKeystoreNotProvided = errors.New("keystore not provided, use --file or --stdin")
)

type Session struct {
	Version string

	store    *mixin.Keystore
	token    string
	pin      string
	spendKey *mixinnet.Key
}

func (s *Session) WithKeystore(store *mixin.Keystore) *Session {
	s.store = store
	return s
}

func (s *Session) WithAccessToken(token string) *Session {
	s.token = token
	return s
}

func (s *Session) WithPin(pin string) *Session {
	s.pin = pin
	return s
}

func (s *Session) WithSpendKey(key *mixinnet.Key) *Session {
	s.spendKey = key
	return s
}

func (s *Session) GetKeystore() (*mixin.Keystore, error) {
	if s.store != nil {
		return s.store, nil
	}

	return nil, ErrKeystoreNotProvided
}

func (s *Session) GetPin() string {
	return s.pin
}

func (s *Session) GetSpendKey() *mixinnet.Key {
	return s.spendKey
}

func (s *Session) GetClient() (*mixin.Client, error) {
	store, err := s.GetKeystore()
	if err != nil {
		return mixin.NewFromAccessToken(s.token), nil
	}

	return mixin.NewFromKeystore(store)
}
