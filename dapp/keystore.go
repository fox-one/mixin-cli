package dapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type KeyStore struct {
	UserID     string `json:"client_id,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	PinToken   string `json:"pin_token,omitempty"`
	Pin        string `json:"pin,omitempty"`
}

func LoadKeyStoreFromFile(file string) (*KeyStore, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("open keystore file %s failed:%w", file, err)
	}
	defer f.Close()

	var store KeyStore
	if err := json.NewDecoder(f).Decode(&store); err != nil {
		return nil, fmt.Errorf("json decode %s failed", file)
	}

	return &store, nil
}

const keyStoreFileExt = ".json"

func SearchKeyStoreFiles(root string, name string) (files []string) {
	if filepath.Ext(name) == keyStoreFileExt {
		files = append(files, name)
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) != keyStoreFileExt {
			return nil
		}

		base := filepath.Base(path)
		filename := strings.TrimSuffix(base, keyStoreFileExt)
		if name == "" || filename == name {
			files = append(files, path)
		}

		return nil
	})

	return
}
