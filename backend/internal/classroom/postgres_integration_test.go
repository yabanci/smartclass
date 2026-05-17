//go:build integration

package classroom_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/platform/testsupport"
	"smartclass/internal/user"
)

// seedUser inserts a minimal user needed as FK target for classrooms.created_by.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	const q = `INSERT INTO users (id, email, password_hash, full_name, role)
	            VALUES ($1, $2, $3, $4, $5)`
	_, err := pool.Exec(ctx, q,
		id, id.String()+"@test.local", "ph", "Seed User", string(user.RoleTeacher),
	)
	require.NoError(t, err)
	return id
}

func newClassroom(createdBy uuid.UUID) *classroom.Classroom {
	return &classroom.Classroom{
		Name:        "Room A",
		Description: "Integration test classroom",
		CreatedBy:   createdBy,
	}
}

func TestPostgresClassroomRepository_Integration(t *testing.T) {
	pool, cleanup := testsupport.StartPostgres(t)
	defer cleanup()

	ctx := context.Background()
	repo := classroom.NewPostgresRepository(pool)

	t.Run("Create assigns ID and auto-adds creator as member", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))
		assert.NotEqual(t, uuid.Nil, c.ID)

		// Creator must appear in Members without an explicit Assign call.
		members, err := repo.Members(ctx, c.ID)
		require.NoError(t, err)
		assert.Contains(t, members, ownerID)
	})

	t.Run("GetByID returns the created classroom", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))

		got, err := repo.GetByID(ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, c.ID, got.ID)
		assert.Equal(t, "Room A", got.Name)
		assert.Equal(t, ownerID, got.CreatedBy)
	})

	t.Run("GetByID on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		_, err := repo.GetByID(ctx, uuid.New())
		assert.ErrorIs(t, err, classroom.ErrNotFound)
	})

	t.Run("Update persists new name and description", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))

		c.Name = "Updated Room"
		c.Description = "new description"
		require.NoError(t, repo.Update(ctx, c))

		got, err := repo.GetByID(ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Room", got.Name)
		assert.Equal(t, "new description", got.Description)
	})

	t.Run("Update on missing ID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		err := repo.Update(ctx, &classroom.Classroom{ID: uuid.New(), Name: "x"})
		assert.ErrorIs(t, err, classroom.ErrNotFound)
	})

	t.Run("Delete removes classroom; GetByID returns ErrNotFound", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))

		require.NoError(t, repo.Delete(ctx, c.ID))
		_, err := repo.GetByID(ctx, c.ID)
		assert.ErrorIs(t, err, classroom.ErrNotFound)
	})

	t.Run("Delete cascades to classroom_users", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		memberID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))
		require.NoError(t, repo.Assign(ctx, c.ID, memberID))

		require.NoError(t, repo.Delete(ctx, c.ID))

		// After cascade delete, IsMember must not error — just return false.
		ok, err := repo.IsMember(ctx, c.ID, memberID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Assign / Unassign / IsMember round-trip", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		memberID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))

		require.NoError(t, repo.Assign(ctx, c.ID, memberID))
		ok, err := repo.IsMember(ctx, c.ID, memberID)
		require.NoError(t, err)
		assert.True(t, ok)

		require.NoError(t, repo.Unassign(ctx, c.ID, memberID))
		ok, err = repo.IsMember(ctx, c.ID, memberID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Assign is idempotent (ON CONFLICT DO NOTHING)", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		ownerID := seedUser(t, ctx, pool)
		c := newClassroom(ownerID)
		require.NoError(t, repo.Create(ctx, c))

		// Owner already assigned by Create — second Assign must not error.
		assert.NoError(t, repo.Assign(ctx, c.ID, ownerID))
	})

	t.Run("ListForUser only returns classrooms the user belongs to", func(t *testing.T) {
		testsupport.ResetDB(t, pool)

		owner := seedUser(t, ctx, pool)
		stranger := seedUser(t, ctx, pool)
		c := newClassroom(owner)
		require.NoError(t, repo.Create(ctx, c))

		list, err := repo.ListForUser(ctx, owner, 10, 0)
		require.NoError(t, err)
		assert.Len(t, list, 1)

		list, err = repo.ListForUser(ctx, stranger, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, list)
	})
}
