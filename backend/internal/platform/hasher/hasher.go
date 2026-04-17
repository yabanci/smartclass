package hasher

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Hasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

type Bcrypt struct {
	cost int
}

func NewBcrypt(cost int) *Bcrypt {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &Bcrypt{cost: cost}
}

func (b *Bcrypt) Hash(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), b.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: hash: %w", err)
	}
	return string(h), nil
}

func (b *Bcrypt) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
