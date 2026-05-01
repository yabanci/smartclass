package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestPrincipalSlot_RoundTrip(t *testing.T) {
	slot := &PrincipalSlot{}
	ctx := WithPrincipalSlot(context.Background(), slot)

	got := PrincipalSlotFrom(ctx)
	require.NotNil(t, got)
	assert.False(t, got.Set, "fresh slot starts unset")

	uid := uuid.New()
	got.Principal = Principal{UserID: uid, Role: "teacher"}
	got.Set = true

	again := PrincipalSlotFrom(ctx)
	require.True(t, again.Set, "the slot is a shared pointer; writes from any holder must be visible to all")
	assert.Equal(t, uid, again.Principal.UserID)
	assert.Equal(t, "teacher", again.Principal.Role)
}

func TestPrincipalSlotFrom_ReturnsNilWhenAbsent(t *testing.T) {
	got := PrincipalSlotFrom(context.Background())
	assert.Nil(t, got, "callers without WithPrincipalSlot must safely get nil — no panic")
}

func TestRequestLogger_IncludesUserIDAfterAuthnImitator(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	authnImitator := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slot := PrincipalSlotFrom(r.Context()); slot != nil {
				slot.Principal = Principal{
					UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					Role:   "teacher",
				}
				slot.Set = true
			}
			next.ServeHTTP(w, r)
		})
	}

	handler := RequestLogger(logger)(authnImitator(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, 1, logs.Len(), "exactly one http log line per request")
	entry := logs.All()[0]
	// Zap's observer keeps Stringer fields on f.Interface (not f.String).
	// Convert each field to a comparable string regardless of zap's internal type.
	fields := map[string]string{}
	for _, f := range entry.Context {
		switch v := f.Interface.(type) {
		case nil:
			fields[f.Key] = f.String
		case interface{ String() string }:
			fields[f.Key] = v.String()
		default:
			fields[f.Key] = f.String
		}
	}
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", fields["user_id"],
		"user_id must be on the log line — that's the whole point of the slot pattern")
	assert.Equal(t, "teacher", fields["role"])
}
