package main

import (
	"encoding/json"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "relix"
	keyringUser    = "gitlab-credentials"
)

// LoadCredentials retrieves credentials from the system keyring
func LoadCredentials() (*Credentials, error) {
	secret, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal([]byte(secret), &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// SaveCredentials stores credentials in the system keyring
func SaveCredentials(creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, keyringUser, string(data))
}

// DeleteCredentials removes credentials from the system keyring
func DeleteCredentials() error {
	return keyring.Delete(keyringService, keyringUser)
}
