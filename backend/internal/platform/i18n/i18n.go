package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Lang string

const (
	EN Lang = "en"
	RU Lang = "ru"
	KZ Lang = "kz"
)

var supported = map[Lang]struct{}{EN: {}, RU: {}, KZ: {}}

func ParseLang(s string) Lang {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, ",-;"); i >= 0 {
		s = s[:i]
	}
	switch Lang(s) {
	case EN, RU, KZ:
		return Lang(s)
	}
	return KZ
}

type Bundle struct {
	mu      sync.RWMutex
	default_ Lang
	messages map[Lang]map[string]string
}

func NewBundle(def Lang) *Bundle {
	if _, ok := supported[def]; !ok {
		def = KZ
	}
	return &Bundle{
		default_: def,
		messages: make(map[Lang]map[string]string),
	}
}

func (b *Bundle) Add(lang Lang, messages map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.messages[lang]
	if !ok {
		m = make(map[string]string, len(messages))
		b.messages[lang] = m
	}
	for k, v := range messages {
		m[k] = v
	}
}

func (b *Bundle) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("i18n: read dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		lang := Lang(strings.TrimSuffix(e.Name(), ".json"))
		if _, ok := supported[lang]; !ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("i18n: read %s: %w", e.Name(), err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("i18n: parse %s: %w", e.Name(), err)
		}
		b.Add(lang, m)
	}
	return nil
}

func MustLoadDir(dir string) *Bundle {
	b := NewBundle(KZ)
	if err := b.LoadDir(dir); err != nil {
		panic(err)
	}
	return b
}

func (b *Bundle) T(lang Lang, key string, args ...any) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if m, ok := b.messages[lang]; ok {
		if v, ok := m[key]; ok {
			return maybeFormat(v, args...)
		}
	}
	if m, ok := b.messages[b.default_]; ok {
		if v, ok := m[key]; ok {
			return maybeFormat(v, args...)
		}
	}
	return key
}

func maybeFormat(tmpl string, args ...any) string {
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

type ctxKey struct{}

func WithLang(ctx context.Context, l Lang) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func LangFrom(ctx context.Context) Lang {
	if v, ok := ctx.Value(ctxKey{}).(Lang); ok {
		return v
	}
	return KZ
}

var ErrUnsupportedLang = errors.New("unsupported language")
