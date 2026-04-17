package classroom_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/classroom"
	"smartclass/internal/classroom/classroomtest"
	"smartclass/internal/user"
)

func newSvc() (*classroom.Service, *classroomtest.MemRepo) {
	repo := classroomtest.NewMemRepo()
	return classroom.NewService(repo), repo
}

func TestService_Create_AutoAssignsCreator(t *testing.T) {
	svc, repo := newSvc()
	creator := uuid.New()
	c, err := svc.Create(context.Background(), classroom.CreateInput{Name: "101", CreatedBy: creator})
	require.NoError(t, err)
	assert.Equal(t, "101", c.Name)

	ok, err := repo.IsMember(context.Background(), c.ID, creator)
	require.NoError(t, err)
	assert.True(t, ok, "creator should be auto-assigned")
}

func TestService_ListForPrincipal(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()

	alice := uuid.New()
	bob := uuid.New()
	admin := uuid.New()

	a, _ := svc.Create(ctx, classroom.CreateInput{Name: "A", CreatedBy: alice})
	b, _ := svc.Create(ctx, classroom.CreateInput{Name: "B", CreatedBy: bob})

	t.Run("teacher sees only their own", func(t *testing.T) {
		list, err := svc.ListForPrincipal(ctx, classroom.Principal{UserID: alice, Role: user.RoleTeacher}, 50, 0)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, a.ID, list[0].ID)
	})

	t.Run("admin sees all", func(t *testing.T) {
		list, err := svc.ListForPrincipal(ctx, classroom.Principal{UserID: admin, Role: user.RoleAdmin}, 50, 0)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})

	_ = repo
	_ = b
}

func TestService_Authorize(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()

	owner := uuid.New()
	member := uuid.New()
	outsider := uuid.New()

	c, err := svc.Create(ctx, classroom.CreateInput{Name: "X", CreatedBy: owner})
	require.NoError(t, err)
	require.NoError(t, svc.Assign(ctx, classroom.Principal{UserID: owner, Role: user.RoleTeacher}, c.ID, member))

	cases := []struct {
		name    string
		p       classroom.Principal
		mutate  bool
		wantErr error
	}{
		{"owner read", classroom.Principal{UserID: owner, Role: user.RoleTeacher}, false, nil},
		{"owner mutate", classroom.Principal{UserID: owner, Role: user.RoleTeacher}, true, nil},
		{"member read", classroom.Principal{UserID: member, Role: user.RoleTeacher}, false, nil},
		{"member mutate denied", classroom.Principal{UserID: member, Role: user.RoleTeacher}, true, classroom.ErrForbidden},
		{"outsider denied", classroom.Principal{UserID: outsider, Role: user.RoleTeacher}, false, classroom.ErrForbidden},
		{"admin ok", classroom.Principal{UserID: outsider, Role: user.RoleAdmin}, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Authorize(ctx, tc.p, c.ID, tc.mutate)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_Update_Delete(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	c, _ := svc.Create(ctx, classroom.CreateInput{Name: "Old", CreatedBy: owner})

	name := "New"
	u, err := svc.Update(ctx, classroom.Principal{UserID: owner, Role: user.RoleTeacher}, c.ID, classroom.UpdateInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "New", u.Name)

	require.NoError(t, svc.Delete(ctx, classroom.Principal{UserID: owner, Role: user.RoleTeacher}, c.ID))

	_, err = svc.Get(ctx, c.ID)
	assert.ErrorIs(t, err, classroom.ErrDomainNotFound)
}
