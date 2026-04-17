package validation

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	v *validator.Validate
}

func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	return &Validator{v: v}
}

type FieldError struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Param string `json:"param,omitempty"`
}

type Errors struct {
	Fields []FieldError
}

func (e *Errors) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+":"+f.Rule)
	}
	return "validation: " + strings.Join(parts, ", ")
}

func (v *Validator) Struct(s any) error {
	err := v.v.Struct(s)
	if err == nil {
		return nil
	}
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err
	}
	out := &Errors{Fields: make([]FieldError, 0, len(ve))}
	for _, fe := range ve {
		out.Fields = append(out.Fields, FieldError{
			Field: lowerFirst(fe.Field()),
			Rule:  fe.Tag(),
			Param: fe.Param(),
		})
	}
	return out
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
