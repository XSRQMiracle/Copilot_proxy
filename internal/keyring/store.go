package keyring

import (
	"errors"
	"fmt"

	oskeyring "github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("token not found in system keyring")

type Store struct {
	service string
	account string
}

func New(service, account string) Store {
	return Store{service: service, account: account}
}

func (s Store) Get() (string, error) {
	token, err := oskeyring.Get(s.service, s.account)
	if errors.Is(err, oskeyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read system keyring: %w", err)
	}
	return token, nil
}

func (s Store) Set(token string) error {
	if err := oskeyring.Set(s.service, s.account, token); err != nil {
		return fmt.Errorf("write system keyring: %w", err)
	}
	return nil
}

func (s Store) Delete() error {
	if err := oskeyring.Delete(s.service, s.account); err != nil && !errors.Is(err, oskeyring.ErrNotFound) {
		return fmt.Errorf("delete system keyring item: %w", err)
	}
	return nil
}
