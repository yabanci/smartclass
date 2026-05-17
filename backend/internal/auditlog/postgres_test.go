package auditlog

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestInsertSQLColCount verifies that for N entries buildInsertSQL generates
// exactly N*insertColCount '$' placeholders, and that buildInsertArgs returns
// exactly N*insertColCount arguments. This is a compile-time-style guard that
// catches any future mismatch between insertColumns and buildInsertArgs.
func TestInsertSQLColCount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		entries []Entry
		wantN   int
	}{
		{
			name:    "single entry produces insertColCount placeholders",
			entries: makeEntries(1),
			wantN:   insertColCount,
		},
		{
			name:    "two entries produce 2*insertColCount placeholders",
			entries: makeEntries(2),
			wantN:   2 * insertColCount,
		},
		{
			name:    "five entries produce 5*insertColCount placeholders",
			entries: makeEntries(5),
			wantN:   5 * insertColCount,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sql := buildInsertSQL(len(tc.entries))

			got := strings.Count(sql, "$")
			if got != tc.wantN {
				t.Errorf("buildInsertSQL(%d): got %d '$' placeholders, want %d\nSQL: %s",
					len(tc.entries), got, tc.wantN, sql)
			}

			args, err := buildInsertArgs(tc.entries)
			if err != nil {
				t.Fatalf("buildInsertArgs: %v", err)
			}
			if len(args) != tc.wantN {
				t.Errorf("buildInsertArgs(%d): got %d args, want %d",
					len(tc.entries), len(args), tc.wantN)
			}
		})
	}
}

// TestInsertSQLFor2EntriesHas12Placeholders is an explicit regression test
// anchored at N=2 / 12 placeholders matching the original bug report.
func TestInsertSQLFor2EntriesHas12Placeholders(t *testing.T) {
	t.Parallel()

	sql := buildInsertSQL(2)
	got := strings.Count(sql, "$")
	const want = 12
	if got != want {
		t.Errorf("buildInsertSQL(2): got %d '$' placeholders, want %d\nSQL: %s", got, want, sql)
	}
}

func makeEntries(n int) []Entry {
	out := make([]Entry, n)
	for i := range out {
		actor := uuid.New()
		entity := uuid.New()
		out[i] = Entry{
			ActorID:    &actor,
			EntityType: EntityDevice,
			EntityID:   &entity,
			Action:     ActionCreate,
			Metadata:   map[string]any{"k": "v"},
			CreatedAt:  time.Now().UTC(),
		}
	}
	return out
}
