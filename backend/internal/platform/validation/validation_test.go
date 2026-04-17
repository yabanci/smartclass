package validation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Email string `validate:"required,email"`
	Name  string `validate:"required,min=2"`
}

func TestValidator_Struct_OK(t *testing.T) {
	v := New()
	require.NoError(t, v.Struct(sample{Email: "a@b.co", Name: "Al"}))
}

func TestValidator_Struct_FieldErrors(t *testing.T) {
	v := New()
	err := v.Struct(sample{Email: "bad", Name: "x"})
	require.Error(t, err)

	var ve *Errors
	require.True(t, errors.As(err, &ve))
	assert.Len(t, ve.Fields, 2)

	fields := map[string]string{}
	for _, f := range ve.Fields {
		fields[f.Field] = f.Rule
	}
	assert.Equal(t, "email", fields["email"])
	assert.Equal(t, "min", fields["name"])
}
