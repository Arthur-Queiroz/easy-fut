package store

import (
	"context"
	"testing"
	"time"
)

func TestMaterializeRecurringCreates4AheadIdempotent(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()

	rec, err := s.CreateRecurringMatch(ctx, CreateRecurringInput{
		Weekday: 0, StartTime: "11:00", EndTime: "12:30",
		Venue: "Arena Teste", Address: "Rua 1", SameDayDeadlineTime: "10:30",
	}, votingTestUser(t, s, "active"))
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.MaterializeRecurringMatches(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("primeira materialização criou %d, queria 4", n)
	}

	// segundo tick: nada novo
	n, err = s.MaterializeRecurringMatches(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("materialização idempotente falhou: criou %d no 2º tick", n)
	}

	// partidas criadas corretas: 4 domingos futuros, open, deadline herdada
	rows, err := s.pool.Query(ctx, `
		select match_date::text, status, to_char(confirmation_deadline, 'HH24:MI'), notes, venue
		from matches where recurring_id = $1 order by match_date`, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var date, status, deadline, notes, venue string
		if err := rows.Scan(&date, &status, &deadline, &notes, &venue); err != nil {
			t.Fatal(err)
		}
		count++
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			t.Fatal(err)
		}
		if d.Weekday() != time.Sunday {
			t.Errorf("partida %s caiu em %s, queria domingo", date, d.Weekday())
		}
		if status != "open" {
			t.Errorf("status = %s, queria open", status)
		}
		if deadline != "10:30" {
			t.Errorf("deadline = %s, queria 10:30 (UTC: 13:30 = 10:30 SP)", deadline)
		}
		if notes != "Partida recorrente" {
			t.Errorf("notes = %q", notes)
		}
		if venue != "Arena Teste" {
			t.Errorf("venue = %q", venue)
		}
	}
	if count != 4 {
		t.Fatalf("count = %d, queria 4", count)
	}
}

func TestMaterializeRespectsInactiveAndDelete(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	uid := votingTestUser(t, s, "active")
	rec, err := s.CreateRecurringMatch(ctx, CreateRecurringInput{
		Weekday: 2, StartTime: "19:00", EndTime: "20:30", Venue: "Q", Address: "", SameDayDeadlineTime: "",
	}, uid)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ActivateRecurringMatch(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	n, _ := s.MaterializeRecurringMatches(ctx, 4)
	if n != 0 {
		t.Fatalf("template inativo gerou %d partidas", n)
	}
	if err := s.ActivateRecurringMatch(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	n, _ = s.MaterializeRecurringMatches(ctx, 4)
	if n != 4 {
		t.Fatalf("reativado gerou %d, queria 4", n)
	}
	// delete: partidas ficam, template some; nova materialização não recria (recurring_id limpo)
	rows, _ := s.pool.Query(ctx, `select id from matches where recurring_id = $1`, rec.ID)
	ids, _ := collectIDs(rows)
	if err := s.DeleteRecurringMatch(ctx, rec.ID); err != nil {
		t.Fatal(err)
	}
	var n2 int
	if err := s.pool.QueryRow(ctx, `select count(*) from matches where id = any($1)`, ids).Scan(&n2); err != nil {
		t.Fatal(err)
	}
	if n2 != len(ids) {
		t.Fatalf("partidas geradas sumiram no delete: %d de %d", n2, len(ids))
	}
	n, _ = s.MaterializeRecurringMatches(ctx, 4)
	if n != 0 {
		t.Fatalf("template excluído voltou a gerar: %d", n)
	}
}

func TestRecurringSeedCreatesOnce(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	// seed precisa de um admin
	votingTestUser(t, s, "active") // garante pool viva
	if _, err := s.pool.Exec(ctx, `
		insert into users (name, email, password_hash, role)
		values ('Admin Seed', 'seed-admin@example.invalid', 'x', 'admin')`); err != nil {
		t.Fatal(err)
	}
	if err := s.RecurringSeed(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, `select count(*) from recurring_matches where weekday=0 and venue=$1`, "Arena Futiba 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("seed criou %d templates, queria 1", n)
	}
	// segunda chamada não duplica
	if err := s.RecurringSeed(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `select count(*) from recurring_matches where weekday=0 and venue=$1`, "Arena Futiba 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("seed duplicou template: %d", n)
	}
}
