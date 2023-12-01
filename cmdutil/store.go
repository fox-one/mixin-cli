package cmdutil

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/fox-one/mixin-sdk-go/v2"
	"github.com/fox-one/mixin-sdk-go/v2/mixinnet"
	jsoniter "github.com/json-iterator/go"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type Keystore struct {
	*mixin.Keystore
	Pin      string        `json:"pin,omitempty"`
	SpendKey *mixinnet.Key `json:"spend_key,omitempty"`
}

func DecodeKeystore(b []byte) (*Keystore, error) {
	var store Keystore
	if err := json.Unmarshal(b, &store); err != nil {
		return nil, fmt.Errorf("json decode keystore failed: %w", err)
	}

	return &store, nil
}

func (store *Keystore) String() string {
	b, _ := json.MarshalIndent(store, "", "  ")
	return string(b)
}

func LookupAndLoadKeystore(name string) ([]byte, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName(name)
	v.SetConfigType("json")
	v.SetConfigType("yaml")
	v.AddConfigPath(path.Join(home, ".mixin-cli"))

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return jsoniter.Marshal(v.AllSettings())
}
