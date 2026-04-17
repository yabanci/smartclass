package hasher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBcrypt_HashCompare(t *testing.T) {
	h := NewBcrypt(4)

	hash, err := h.Hash("secret-pass-42")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "secret-pass-42", hash)

	assert.NoError(t, h.Compare(hash, "secret-pass-42"))
	assert.Error(t, h.Compare(hash, "wrong"))
}

func TestBcrypt_InvalidCostFallsBack(t *testing.T) {
	h := NewBcrypt(100)
	hash, err := h.Hash("x")
	require.NoError(t, err)
	require.NoError(t, h.Compare(hash, "x"))
}

func TestBcrypt_DifferentHashesForSameInput(t *testing.T) {
	h := NewBcrypt(4)
	h1, err := h.Hash("same")
	require.NoError(t, err)
	h2, err := h.Hash("same")
	require.NoError(t, err)
	assert.NotEqual(t, h1, h2, "bcrypt salts should differ")
}
