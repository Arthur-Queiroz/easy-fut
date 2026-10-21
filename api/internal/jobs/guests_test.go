package jobs

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/store"
)

// jobsTestStore sobe um schema isolado com as migrations, como o
// newTestStore do pacote store. Sem TEST_DATABASE_URL, o teste é pulado.
func jobsTestStore(t *testing.T) (*store.Store, *pgxpool.Pool) {
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
	if _, err := root.Exec(ctx, `create extension if not exists pgcrypto with schema public`); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			root.Close()
			t.Fatal(err)
		}
	}
	schema := "jobs_test_" + uuid.NewString()[:8]
	if _, err := root.Exec(ctx, `create schema `+schema); err != nil {
		root.Close()
		t.Fatal(err)
	}
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	st, err := store.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		st.Close()
		raw.Close()
		_, err := root.Exec(ctx, `drop schema `+schema+` cascade`)
		root.Close()
		if err != nil {
			t.Error(err)
		}
	})
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return st, raw
}

// O job de cada minuto expira as reservas vencidas e não toca nas válidas.
func TestExpireGuestBookingsJob(t *testing.T) {
	st, raw := jobsTestStore(t)
	ctx := context.Background()
	var admin string
	if err := raw.QueryRow(ctx, `insert into users (name, email, password_hash, role)
		values ('Admin', 'admin@example.invalid', 'x', 'admin') returning id`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	m, err := st.CreateMatch(ctx, store.MatchInput{
		MatchDate: "2099-01-10", StartTime: "20:00", EndTime: "22:00", Venue: "Arena",
		ConfirmationDeadline: time.Now().Add(2 * time.Hour),
	}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetGuestSale(ctx, m.ID, true, 5, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	book := func(phone string, now time.Time) string {
		t.Helper()
		b, err := st.CreateGuestBooking(ctx, store.GuestBookingInput{
			MatchID: m.ID, Name: "Avulso", Phone: phone, Email: "a@example.invalid", AccessTokenHash: phone,
		}, now)
		if err != nil {
			t.Fatal(err)
		}
		return b.ID
	}
	old := book("11987650001", time.Now().Add(-31*time.Minute))
	fresh := book("11987650002", time.Now())

	New(st, nil, "https://example.invalid").expireGuestBookings()

	for id, want := range map[string]string{old: guests.StatusExpired, fresh: guests.StatusReserved} {
		b, err := st.GuestBookingByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if b.Status != want {
			t.Errorf("inscrição %s = %s, want %s", id, b.Status, want)
		}
	}
}
