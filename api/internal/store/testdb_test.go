package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ensureTestExtensions instala o pgcrypto em public antes das migrations.
// Sem isso, o "create extension if not exists" da 001 cria a extensão no
// schema do teste (o primeiro do search_path), que é apagado no fim, e dois
// pacotes de teste migrando ao mesmo tempo disputam a criação (23505).
func ensureTestExtensions(ctx context.Context, root *pgxpool.Pool) error {
	_, err := root.Exec(ctx, `create extension if not exists pgcrypto with schema public`)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil // outro pacote criou no mesmo instante
	}
	return err
}

// newTestStore cria um schema isolado com as migrations aplicadas e o remove
// no fim do teste; nenhum registro da aplicação é lido ou alterado.
func newTestStore(t *testing.T, prefix string) *Store {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureTestExtensions(ctx, root); err != nil {
		root.Close()
		t.Fatal(err)
	}
	schema := prefix + "_" + uuid.NewString()[:8]
	if _, err = root.Exec(ctx, `create schema `+schema); err != nil {
		root.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	st := &Store{pool: pool}
	t.Cleanup(func() {
		pool.Close()
		_, err := root.Exec(ctx, `drop schema `+schema+` cascade`)
		root.Close()
		if err != nil {
			t.Error(err)
		}
	})
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return st
}
