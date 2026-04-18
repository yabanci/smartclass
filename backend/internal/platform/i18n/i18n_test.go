package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLang(t *testing.T) {
	cases := map[string]Lang{
		"":            KZ,
		"en":          EN,
		"EN":          EN,
		"ru":          RU,
		"kz":          KZ,
		"ru-RU":       RU,
		"en,ru;q=0.9": EN,
		"fr":          KZ,
	}
	for in, want := range cases {
		assert.Equal(t, want, ParseLang(in), "input=%q", in)
	}
}

func TestBundle_T_FallsBackToDefault(t *testing.T) {
	b := NewBundle(EN)
	b.Add(EN, map[string]string{"hello": "Hello"})
	b.Add(RU, map[string]string{"hello": "Привет"})

	assert.Equal(t, "Привет", b.T(RU, "hello"))
	assert.Equal(t, "Hello", b.T(KZ, "hello"), "missing key falls back to default lang")
	assert.Equal(t, "unknown", b.T(EN, "unknown"), "missing everywhere returns key")
}

func TestBundle_T_Format(t *testing.T) {
	b := NewBundle(EN)
	b.Add(EN, map[string]string{"greet": "Hello, %s!"})
	assert.Equal(t, "Hello, Arsen!", b.T(EN, "greet", "Arsen"))
}

func TestLangCtx(t *testing.T) {
	ctx := WithLang(context.Background(), RU)
	assert.Equal(t, RU, LangFrom(ctx))
	assert.Equal(t, KZ, LangFrom(context.Background()))
}
