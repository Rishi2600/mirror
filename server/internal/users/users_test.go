package users

import (
	"context"
	"errors"
	"testing"

	"mirror/server/internal/dbtest"
)

func TestCreateAndFindByEmail(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)

	created, err := Create(ctx, tx, "  Ada.Test@Example.com ", RoleCandidate)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Errorf("Create did not fill ID and CreatedAt: %+v", created)
	}
	if created.Email != "ada.test@example.com" {
		t.Errorf("Email = %q, want it trimmed and lowercase", created.Email)
	}

	found, err := ByEmail(ctx, tx, "ADA.TEST@example.com")
	if err != nil {
		t.Fatalf("ByEmail: %v", err)
	}
	if found.ID != created.ID || found.Role != RoleCandidate {
		t.Errorf("ByEmail = %+v, want %+v", found, created)
	}
}

func TestDuplicateEmailIsRejected(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)

	if _, err := Create(ctx, tx, "dup@example.com", RoleEmployer); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := Create(ctx, tx, "DUP@example.com", RoleCandidate)
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("second Create error = %v, want ErrEmailTaken", err)
	}
}

func TestUnknownEmailIsNotFound(t *testing.T) {
	_, err := ByEmail(context.Background(), dbtest.Tx(t), "nobody@example.com")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestUnknownRoleIsRejected(t *testing.T) {
	_, err := Create(context.Background(), dbtest.Tx(t), "role@example.com", Role("superuser"))
	if err == nil {
		t.Error("Create accepted an unknown role")
	}
}

func TestRoleIsPickedOnce(t *testing.T) {
	ctx := context.Background()
	tx := dbtest.Tx(t)

	u, err := Create(ctx, tx, "new@example.com", "")
	if err != nil {
		t.Fatalf("Create without a role: %v", err)
	}
	if u, _ = ByID(ctx, tx, u.ID); u.Role != "" {
		t.Errorf("Role = %q, want empty", u.Role)
	}

	if err := SetRole(ctx, tx, u.ID, RoleEmployer); err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if u, _ = ByID(ctx, tx, u.ID); u.Role != RoleEmployer {
		t.Errorf("Role = %q, want employer", u.Role)
	}
	if err := SetRole(ctx, tx, u.ID, RoleCandidate); !errors.Is(err, ErrRoleSet) {
		t.Errorf("second SetRole error = %v, want ErrRoleSet", err)
	}
}
