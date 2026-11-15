//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/natnael-alemayehu/kiflet/migrations"
)

// TestMigrations: the migrations build the schema, each can run a second
// time without changing it, and they come down to nothing and up again to
// the same schema. A database is migrated when the app starts, so a
// migration that half ran must be safe to run again.
func TestMigrations(t *testing.T) {
	ctx := context.Background()
	conn, db := scratchDatabase(t)

	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("up: %v", err)
	}
	built := schemaOf(t, conn)
	if !strings.Contains(built, "table shipments") || !strings.Contains(built, "trigger CREATE TRIGGER audit_logs_append_only") {
		t.Fatalf("the migrations built something unexpected:\n%s", built)
	}

	files, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations: %v", err)
	}
	for _, name := range files {
		raw, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(ctx, upSection(string(raw))); err != nil {
			t.Fatalf("%s again: %v", name, err)
		}
		if after := schemaOf(t, conn); after != built {
			t.Fatalf("%s run again changed the schema:\n%s", name, schemaDiff(built, after))
		}
	}

	if err := goose.DownToContext(ctx, db, ".", 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if left := schemaOf(t, conn); left != "" {
		t.Fatalf("the migrations' Down sections left:\n%s", left)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if again := schemaOf(t, conn); again != built {
		t.Fatalf("up after down built a different schema:\n%s", schemaDiff(built, again))
	}
}

// scratchDatabase makes an empty database beside the tests' own, and
// removes it when the test ends.
func scratchDatabase(t *testing.T) (*pgx.Conn, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("kiflet_migrations_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := pgx.ParseConfig(u.String())
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() {
		db.Close()
		conn.Close(ctx)
		if _, err := pool.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("removing %s: %v", name, err)
		}
	})
	return conn, db
}

// upSection is a migration file's Up statements, as one script.
func upSection(file string) string {
	_, up, _ := strings.Cut(file, "-- +goose Up")
	up, _, _ = strings.Cut(up, "-- +goose Down")
	var keep []string
	for _, line := range strings.Split(up, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "-- +goose") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// schemaOf describes the schema the migrations made, one line per column,
// constraint, index, trigger, function and sequence, sorted. What belongs
// to an extension and goose's own table are left out. Columns are numbered
// in their order, which the generated code follows.
func schemaOf(t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	var s *string
	err := conn.QueryRow(context.Background(), `
WITH ours AS (
	SELECT c.oid, c.relname, c.relkind FROM pg_class c
	WHERE c.relnamespace = 'public'::regnamespace
	  AND c.relname NOT LIKE 'goose_db_version%'
	  AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = c.oid AND d.deptype = 'e')
)
SELECT string_agg(line, E'\n' ORDER BY line) FROM (
	SELECT 'table ' || relname AS line FROM ours WHERE relkind = 'r'
	UNION ALL
	SELECT format('column %s %s %s %s%s%s%s', o.relname,
		lpad((row_number() OVER (PARTITION BY a.attrelid ORDER BY a.attnum))::text, 3, '0'),
		a.attname, format_type(a.atttypid, a.atttypmod),
		CASE WHEN a.attnotnull THEN ' not null' ELSE '' END,
		COALESCE(' default ' || pg_get_expr(d.adbin, d.adrelid), ''),
		CASE WHEN a.attidentity <> '' THEN ' identity ' || a.attidentity::text ELSE '' END)
	FROM ours o JOIN pg_attribute a ON a.attrelid = o.oid
	LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
	WHERE o.relkind = 'r' AND a.attnum > 0 AND NOT a.attisdropped
	UNION ALL
	SELECT format('constraint %s %s %s', o.relname, k.conname, pg_get_constraintdef(k.oid))
	FROM pg_constraint k JOIN ours o ON o.oid = k.conrelid
	UNION ALL
	SELECT 'index ' || pg_get_indexdef(i.indexrelid)
	FROM pg_index i JOIN ours o ON o.oid = i.indexrelid
	UNION ALL
	SELECT 'trigger ' || pg_get_triggerdef(g.oid)
	FROM pg_trigger g JOIN ours o ON o.oid = g.tgrelid WHERE NOT g.tgisinternal
	UNION ALL
	SELECT 'sequence ' || relname FROM ours WHERE relkind = 'S'
	UNION ALL
	SELECT 'function ' || pg_get_functiondef(p.oid) FROM pg_proc p
	WHERE p.pronamespace = 'public'::regnamespace
	  AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e')
) lines`).Scan(&s)
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		return ""
	}
	return *s
}

// schemaDiff lists the lines only one of two schemas has.
func schemaDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	var out []string
	for _, l := range la {
		if !slices.Contains(lb, l) {
			out = append(out, "- "+l)
		}
	}
	for _, l := range lb {
		if !slices.Contains(la, l) {
			out = append(out, "+ "+l)
		}
	}
	return strings.Join(out, "\n")
}
