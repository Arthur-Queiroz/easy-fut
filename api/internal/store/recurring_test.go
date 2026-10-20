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
		select match_date::text, status, to_char(confirmation_deadline at time zone 'America/Sao_Paulo', 'HH24:MI'), notes, venue
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
			t.Errorf("deadline = %s, queria 10:30 em São Paulo", deadline)
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

func recurringTestTemplate(t *testing.T, s *Store, weekday int, start, deadline string) RecurringMatch {
	t.Helper()
	rec, err := s.CreateRecurringMatch(context.Background(), CreateRecurringInput{
		Weekday: weekday, StartTime: start, EndTime: "23:00",
		Venue: "Arena Teste", SameDayDeadlineTime: deadline,
	}, votingTestUser(t, s, "active"))
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func spToday(t *testing.T) time.Time {
	t.Helper()
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	y, m, d := time.Now().In(sp).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Qualquer que seja o dia em que o job roda, cada template ganha exatamente
// `ahead` partidas, nas próximas semanas a partir de amanhã. Antes a janela
// tinha 29 dias e o dia de amanhã aparecia 5 vezes.
func TestMaterializeCreatesExactlyAheadForEveryWeekday(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	templates := make([]RecurringMatch, 7)
	for weekday := range 7 {
		templates[weekday] = recurringTestTemplate(t, s, weekday, "19:00", "")
	}

	n, err := s.MaterializeRecurringMatches(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 28 {
		t.Errorf("criou %d partidas, queria 28 (4 por template)", n)
	}
	today := spToday(t)
	for weekday, rec := range templates {
		rows, err := s.pool.Query(ctx, `select match_date from matches where recurring_id = $1 order by match_date`, rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		var dates []time.Time
		for rows.Next() {
			var d time.Time
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			dates = append(dates, d)
		}
		rows.Close()
		if len(dates) != 4 {
			t.Errorf("weekday %d: %d partidas, queria 4 (%v)", weekday, len(dates), dates)
			continue
		}
		for _, d := range dates {
			if int(d.Weekday()) != weekday || !d.After(today) || d.After(today.AddDate(0, 0, 28)) {
				t.Errorf("weekday %d: data %s fora da janela de amanhã a hoje+28", weekday, d.Format("2006-01-02"))
			}
		}
	}
}

// O horário do template é de São Paulo, qualquer que seja o fuso da sessão do
// PostgreSQL (UTC no container de produção e no de teste).
func TestMaterializeDeadlineIsSaoPauloTime(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	withDeadline := recurringTestTemplate(t, s, 0, "11:00", "10:30")
	fallback := recurringTestTemplate(t, s, 3, "19:00", "")
	if _, err := s.MaterializeRecurringMatches(ctx, 1); err != nil {
		t.Fatal(err)
	}
	sp, _ := time.LoadLocation("America/Sao_Paulo")
	for rec, hhmm := range map[string][2]int{withDeadline.ID: {10, 30}, fallback.ID: {18, 30}} {
		var date, deadline time.Time
		if err := s.pool.QueryRow(ctx, `select match_date, confirmation_deadline from matches where recurring_id = $1`, rec).Scan(&date, &deadline); err != nil {
			t.Fatal(err)
		}
		want := time.Date(date.Year(), date.Month(), date.Day(), hhmm[0], hhmm[1], 0, 0, sp)
		if !deadline.Equal(want) {
			t.Errorf("prazo = %s, queria %s", deadline.In(sp), want)
		}
	}
}

// Como em CreateMatch, os ativos recebem confirmação no_response: é dela que
// saem o lembrete por push e a lista "sem resposta". Quem vira ativo depois
// ganha a linha nas partidas ainda abertas na próxima execução.
func TestMaterializeGivesActiveUsersNoResponse(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	rec := recurringTestTemplate(t, s, 0, "11:00", "10:30")
	creator := rec.CreatedBy
	active := votingTestUser(t, s, "active")
	inactive := votingTestUser(t, s, "inactive")
	if _, err := s.MaterializeRecurringMatches(ctx, 2); err != nil {
		t.Fatal(err)
	}
	rows, err := s.pool.Query(ctx, `select id from matches where recurring_id = $1 order by match_date`, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := collectIDs(rows)
	if err != nil || len(ids) != 2 {
		t.Fatalf("partidas = %v, %v", ids, err)
	}
	response := func(match, user string) string {
		t.Helper()
		var r string
		err := s.pool.QueryRow(ctx, `select coalesce((select response from match_confirmations where match_id = $1 and user_id = $2), '')`, match, user).Scan(&r)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, id := range ids {
		for user, want := range map[string]string{creator: "no_response", active: "no_response", inactive: ""} {
			if got := response(id, user); got != want {
				t.Errorf("partida %s, usuário %s: %q, queria %q", id, user, got, want)
			}
		}
	}

	// Resposta dada continua; partida fechada não ganha linhas novas.
	if err := s.Confirm(ctx, ids[1], active, "going"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update matches set status = 'closed' where id = $1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	late := votingTestUser(t, s, "active")
	if _, err := s.MaterializeRecurringMatches(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if got := response(ids[1], active); got != "going" {
		t.Errorf("resposta do ativo virou %q", got)
	}
	if got := response(ids[1], late); got != "no_response" {
		t.Errorf("novo ativo na partida aberta: %q, queria no_response", got)
	}
	if got := response(ids[0], late); got != "" {
		t.Errorf("novo ativo na partida fechada: %q, queria nada", got)
	}
}

// 013 corrige os prazos gravados no fuso da sessão (UTC) antes da correção,
// sem tocar em prazo editado pelo admin nem em partida já fechada.
func TestMigrationFixesRecurringDeadlines(t *testing.T) {
	s := votingTestStore(t)
	ctx := context.Background()
	rec := recurringTestTemplate(t, s, 0, "11:00", "10:30")
	insert := func(date, status, deadline string) string {
		t.Helper()
		var id string
		err := s.pool.QueryRow(ctx, `
			insert into matches (match_date, start_time, end_time, venue, confirmation_deadline, created_by, recurring_id, status)
			values ($1::date, '11:00', '12:30', 'Arena Teste', `+deadline+`, $2, $3, $4) returning id`,
			date, rec.CreatedBy, rec.ID, status).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	buggy := `($1::date + time '10:30')::timestamptz`
	wrong := insert("2099-01-04", "open", buggy)
	edited := insert("2099-01-11", "open", `($1::date + time '09:00') at time zone 'America/Sao_Paulo'`)
	closed := insert("2099-01-18", "closed", buggy)
	before := map[string]time.Time{}
	for _, id := range []string{edited, closed} {
		var d time.Time
		_ = s.pool.QueryRow(ctx, `select confirmation_deadline from matches where id = $1`, id).Scan(&d)
		before[id] = d
	}

	sql, err := migrationsFS.ReadFile("migrations/013_recurring_deadline_tz.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}

	sp, _ := time.LoadLocation("America/Sao_Paulo")
	var got time.Time
	if err := s.pool.QueryRow(ctx, `select confirmation_deadline from matches where id = $1`, wrong).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2099, 1, 4, 10, 30, 0, 0, sp); !got.Equal(want) {
		t.Errorf("prazo corrigido = %s, queria %s", got.In(sp), want)
	}
	for id, want := range before {
		if err := s.pool.QueryRow(ctx, `select confirmation_deadline from matches where id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if !got.Equal(want) {
			t.Errorf("partida %s mudou de %s para %s", id, want, got)
		}
	}
}
