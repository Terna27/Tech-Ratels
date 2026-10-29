package migrator_test

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"ratels-api/internal/migrator"
	"ratels-api/internal/security"
	"ratels-api/migrations"
	"strings"
	"testing"
)

func TestPhase71MigrationForwardBackwardPreservesMembers(t *testing.T) {
	uri := os.Getenv("TEST_DATABASE_URL")
	if uri == "" {
		uri = "postgres://postgres@127.0.0.1:5432/ratels_migration_audit_db?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrator.RunMigrations(ctx, pool); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	raw, err := migrations.FS.ReadFile("20261029000026_member_cards.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("missing rollback")
	}
	if _, err = tx.Exec(ctx, parts[1]); err != nil {
		t.Fatal("rollback", err)
	}
	id, blank := security.GenerateUUID(), security.GenerateUUID()
	ref := "LEGACY-" + security.GenerateUUID()
	_, err = tx.Exec(ctx, `INSERT INTO users(id,password_hash,display_name,membership_reference,status) VALUES($1,'test-only','Preserved',$2,'active'),($3,'test-only','Blank legacy','','inactive')`, id, ref, blank)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, parts[0]); err != nil {
		t.Fatal("forward", err)
	}
	var got, status string
	if err = tx.QueryRow(ctx, `SELECT membership_reference,status FROM users WHERE id=$1`, id).Scan(&got, &status); err != nil || got != ref || status != "active" {
		t.Fatal("existing membership changed", got, status, err)
	}
	if err = tx.QueryRow(ctx, `SELECT membership_reference,status FROM users WHERE id=$1`, blank).Scan(&got, &status); err != nil || got == "" || status != "inactive" {
		t.Fatal("legacy repair", got, status, err)
	}
	if _, err = tx.Exec(ctx, parts[1]); err != nil {
		t.Fatal("rollback after forward", err)
	}
	var n int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=ANY($1::text[])`, []string{id, blank}).Scan(&n); err != nil || n != 2 {
		t.Fatal("rollback deleted members", n, err)
	}
}
