//go:build integration

package auditlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/auditlog"
	"smartclass/internal/platform/testsupport"
)

func ptr[T any](v T) *T { return &v }

func TestPostgresAuditLogRepository_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	repo := auditlog.NewPostgresRepository(pool)

	t.Run("Insert single entry and List retrieves it", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		actorID := uuid.New()
		entityID := uuid.New()
		entry := auditlog.Entry{
			ActorID:    &actorID,
			EntityType: auditlog.EntityDevice,
			EntityID:   &entityID,
			Action:     auditlog.ActionCommand,
			Metadata:   map[string]any{"cmd": "on"},
		}

		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{entry}))

		entries, err := repo.List(ctx, auditlog.Query{Limit: 10})
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, auditlog.EntityDevice, entries[0].EntityType)
		assert.Equal(t, auditlog.ActionCommand, entries[0].Action)
		assert.Equal(t, "on", entries[0].Metadata["cmd"])
	})

	t.Run("Insert batch of entries in a single round-trip", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		const batchSize = 50
		entries := make([]auditlog.Entry, batchSize)
		for i := range entries {
			id := uuid.New()
			entries[i] = auditlog.Entry{
				ActorID:    &id,
				EntityType: auditlog.EntityClassroom,
				Action:     auditlog.ActionCreate,
			}
		}
		require.NoError(t, repo.Insert(ctx, entries))

		got, err := repo.List(ctx, auditlog.Query{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, got, batchSize)
	})

	t.Run("Insert with zero entries is a no-op", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		assert.NoError(t, repo.Insert(ctx, nil))
		assert.NoError(t, repo.Insert(ctx, []auditlog.Entry{}))
	})

	t.Run("List filters by ActorID", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		a1, a2 := uuid.New(), uuid.New()
		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{
			{ActorID: &a1, EntityType: auditlog.EntityUser, Action: auditlog.ActionLogin},
			{ActorID: &a2, EntityType: auditlog.EntityUser, Action: auditlog.ActionLogin},
		}))

		got, err := repo.List(ctx, auditlog.Query{ActorID: &a1, Limit: 10})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, a1, *got[0].ActorID)
	})

	t.Run("List filters by EntityType", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{
			{EntityType: auditlog.EntityDevice, Action: auditlog.ActionCommand},
			{EntityType: auditlog.EntityClassroom, Action: auditlog.ActionCreate},
		}))

		got, err := repo.List(ctx, auditlog.Query{EntityType: auditlog.EntityDevice, Limit: 10})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, auditlog.EntityDevice, got[0].EntityType)
	})

	t.Run("List filters by time range (From/To)", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		now := time.Now().UTC()
		yesterday := now.Add(-24 * time.Hour)
		tomorrow := now.Add(24 * time.Hour)

		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{
			{EntityType: auditlog.EntityScene, Action: auditlog.ActionSceneRun, CreatedAt: yesterday},
			{EntityType: auditlog.EntityScene, Action: auditlog.ActionSceneRun, CreatedAt: now},
			{EntityType: auditlog.EntityScene, Action: auditlog.ActionSceneRun, CreatedAt: tomorrow},
		}))

		// Query only entries from now onward.
		got, err := repo.List(ctx, auditlog.Query{
			From:  ptr(now.Add(-time.Millisecond)),
			Limit: 10,
		})
		require.NoError(t, err)
		assert.Len(t, got, 2, "should return now + tomorrow entries")
	})

	t.Run("List default limit is 100 when Limit is 0", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		entries := make([]auditlog.Entry, 120)
		for i := range entries {
			entries[i] = auditlog.Entry{EntityType: auditlog.EntityUser, Action: auditlog.ActionLogin}
		}
		require.NoError(t, repo.Insert(ctx, entries))

		got, err := repo.List(ctx, auditlog.Query{Limit: 0})
		require.NoError(t, err)
		assert.Len(t, got, 100, "default limit should cap at 100")
	})

	t.Run("List with Offset skips rows", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{
			{EntityType: auditlog.EntityUser, Action: auditlog.ActionCreate},
			{EntityType: auditlog.EntityUser, Action: auditlog.ActionUpdate},
			{EntityType: auditlog.EntityUser, Action: auditlog.ActionDelete},
		}))

		// All three, ordered created_at DESC.
		all, err := repo.List(ctx, auditlog.Query{Limit: 10})
		require.NoError(t, err)
		require.Len(t, all, 3)

		// Skip first row.
		paged, err := repo.List(ctx, auditlog.Query{Limit: 10, Offset: 1})
		require.NoError(t, err)
		assert.Len(t, paged, 2)
		assert.Equal(t, all[1].ID, paged[0].ID)
	})

	t.Run("Metadata JSON round-trips correctly (M-201 column drift guard)", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		meta := map[string]any{
			"key":    "value",
			"number": float64(42),
			"nested": map[string]any{"a": "b"},
		}
		require.NoError(t, repo.Insert(ctx, []auditlog.Entry{
			{EntityType: auditlog.EntityDevice, Action: auditlog.ActionCommand, Metadata: meta},
		}))

		got, err := repo.List(ctx, auditlog.Query{Limit: 1})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "value", got[0].Metadata["key"])
		assert.Equal(t, float64(42), got[0].Metadata["number"])
	})
}
