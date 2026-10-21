package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/push"
)

// isolationGuest cria um avulso ativo (como a inscrição cria) com um aparelho
// inscrito no push, o pior caso para o isolamento: em tudo que fala do grupo,
// ele precisa ser invisível.
func isolationGuest(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := s.pool.QueryRow(ctx, `insert into users (name, phone, kind, role, status)
		values ('Avulso Isolado', '+5511987654321', 'guest', 'player', 'active') returning id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	sub := push.Subscription{Endpoint: "https://fcm.googleapis.com/" + uuid.NewString()}
	sub.Keys.Auth, sub.Keys.P256dh = "auth", "key"
	if err := s.SavePushSubscription(ctx, id, sub); err != nil {
		t.Fatal(err)
	}
	return id
}

func hasConfirmation(t *testing.T, s *Store, matchID, userID string) bool {
	t.Helper()
	var ok bool
	if err := s.pool.QueryRow(context.Background(), `select exists (select 1 from match_confirmations
		where match_id = $1 and user_id = $2)`, matchID, userID).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

func pushQueuedFor(t *testing.T, s *Store, table, userID string) int {
	t.Helper()
	var n int
	if err := s.pool.QueryRow(context.Background(), `select count(*) from `+table+` d
		join push_subscriptions p on p.id = d.subscription_id where p.user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGuestsOutOfMonthlySplit(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")
	member := votingTestUser(t, s, "active")
	guest := isolationGuest(t, s)

	users, err := s.ActiveUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range users {
		if u.ID == guest {
			t.Error("ActiveUsers devolveu o avulso")
		}
	}
	if len(users) != 2 {
		t.Errorf("ActiveUsers = %d, want 2 membros", len(users))
	}

	batch, err := s.GenerateBatch(ctx, "2099-01", 10000, chargesTestDue(), admin, billing.FeeRate{})
	if err != nil {
		t.Fatal(err)
	}
	if batch.UserCount != 2 || batch.IndividualAmountCents != 5000 {
		t.Errorf("rateio = %d × %d, want 2 × 5000", batch.UserCount, batch.IndividualAmountCents)
	}
	var guestCharges int
	if err := s.pool.QueryRow(ctx, `select count(*) from charges where user_id = $1`, guest).Scan(&guestCharges); err != nil {
		t.Fatal(err)
	}
	if guestCharges != 0 {
		t.Errorf("avulso recebeu %d cobranças", guestCharges)
	}

	// Cobrança avulsa de partida também é só para membros.
	if _, err := s.GenerateMatchBatch(ctx, "2099-01", "Quadra", 1000, chargesTestDue(), admin,
		[]string{member, guest}, billing.FeeRate{}); !errors.Is(err, ErrInvalidParticipants) {
		t.Errorf("lote de partida com avulso: %v", err)
	}
}

func TestGuestsOutOfMatchCreationAndPush(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	member, memberSub := pushTestUser(t, s, "active")
	guest := isolationGuest(t, s)

	m := pushTestMatch(t, s, member)
	if !hasConfirmation(t, s, m.ID, member) {
		t.Error("membro ativo sem confirmação inicial")
	}
	if hasConfirmation(t, s, m.ID, guest) {
		t.Error("avulso ganhou confirmação inicial na criação da partida")
	}
	if n := pushQueuedFor(t, s, "match_push_deliveries", guest); n != 0 {
		t.Errorf("push de partida criada para o avulso: %d", n)
	}

	// Mesmo com uma linha no_response (avulso pago e depois corrigido), o
	// lembrete e o envio ignoram o avulso.
	if _, err := s.pool.Exec(ctx, `insert into match_confirmations (match_id, user_id) values ($1, $2)`, m.ID, guest); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update matches set created_at = now() - interval '6 minutes' where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueMatchReminders(ctx, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if n := pushQueuedFor(t, s, "match_push_deliveries", guest); n != 0 {
		t.Errorf("lembrete para o avulso: %d", n)
	}
	// Uma entrega já na fila (criada antes do isolamento) não é enviada.
	var guestSub string
	if err := s.pool.QueryRow(ctx, `select id from push_subscriptions where user_id = $1`, guest).Scan(&guestSub); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `insert into match_push_deliveries (subscription_id, match_id, kind)
		values ($1, $2, 'created')`, guestSub, m.ID); err != nil {
		t.Fatal(err)
	}
	for {
		d, err := s.ClaimPushDelivery(ctx, 2*time.Hour)
		if errors.Is(err, ErrNotFound) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if d.SubscriptionID == guestSub {
			t.Fatal("entrega de push reivindicada para o avulso")
		}
		if d.Subscription.Endpoint != memberSub.Endpoint {
			t.Fatalf("entrega inesperada: %+v", d)
		}
		if err := s.CompletePushDelivery(ctx, d, 201, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGuestsOutOfRecurringMatches(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	recurringTestTemplate(t, s, 0, "11:00", "10:30")
	guest := isolationGuest(t, s)
	if _, err := s.MaterializeRecurringMatches(ctx, 2); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, `select count(*) from match_confirmations where user_id = $1`, guest).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("avulso ganhou %d confirmações em recorrentes", n)
	}
	var members int
	if err := s.pool.QueryRow(ctx, `select count(*) from match_confirmations`).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members == 0 {
		t.Error("membros sem confirmação: o teste não exercitou a materialização")
	}
}

func TestGuestsOutOfBroadcasts(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	admin, _ := pushTestUser(t, s, "active")
	guest := isolationGuest(t, s)

	recipients, err := s.ListPushRecipients(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recipients {
		if r.ID == guest {
			t.Error("avulso na lista de destinatários de avisos")
		}
	}
	if _, err := s.CreatePushBroadcast(ctx, BroadcastInput{Title: "A", Body: "B", Audience: "all"}, admin); err != nil {
		t.Fatal(err)
	}
	if n := pushQueuedFor(t, s, "broadcast_push_deliveries", guest); n != 0 {
		t.Errorf("aviso geral para o avulso: %d", n)
	}
	_, err = s.CreatePushBroadcast(ctx, BroadcastInput{Title: "A", Body: "B", Audience: "selected", UserIDs: []string{guest}}, admin)
	if !errors.Is(err, ErrNoPushRecipients) {
		t.Errorf("aviso escolhido só com o avulso: %v", err)
	}

	// Entrega enfileirada por fora também não sai.
	var guestSub, broadcastID string
	if err := s.pool.QueryRow(ctx, `select id from push_subscriptions where user_id = $1`, guest).Scan(&guestSub); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `select id from push_broadcasts limit 1`).Scan(&broadcastID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `insert into broadcast_push_deliveries (subscription_id, broadcast_id) values ($1, $2)`, guestSub, broadcastID); err != nil {
		t.Fatal(err)
	}
	for _, d := range claimAllBroadcasts(t, s) {
		if d.SubscriptionID == guestSub {
			t.Error("aviso reivindicado para o avulso")
		}
	}
}

func TestGuestsOutOfDashboard(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	votingTestUser(t, s, "active")
	votingTestUser(t, s, "inactive")
	isolationGuest(t, s)
	var inactiveGuest string
	if err := s.pool.QueryRow(ctx, `insert into users (name, kind, status) values ('Avulso inativo', 'guest', 'inactive') returning id`).Scan(&inactiveGuest); err != nil {
		t.Fatal(err)
	}

	d, err := s.DashboardStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.ActiveUsers != 1 || d.InactiveUsers != 1 {
		t.Errorf("painel: %d ativos, %d inativos; want 1 e 1", d.ActiveUsers, d.InactiveUsers)
	}
}

// Inadimplência é regra de mensalista: mesmo com uma cobrança vencida (que
// avulso não deveria ter), o avulso não é inativado nem reativado pelo cron.
func TestGuestsOutOfDelinquency(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	admin := votingTestUser(t, s, "active")
	member := votingTestUser(t, s, "active")
	guest := isolationGuest(t, s)

	batch, err := s.GenerateBatch(ctx, "2099-01", 10000, chargesTestDue(), admin, billing.FeeRate{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `insert into charges (batch_id, user_id, reference_month, amount_cents, due_date)
		values ($1, $2, '2099-01-01', 100, current_date - 10)`, batch.ID, guest); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `update charges set status = 'overdue', due_date = current_date - 10`); err != nil {
		t.Fatal(err)
	}

	users, err := s.UsersToInactivate(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, u := range users {
		found[u.ID] = true
	}
	if !found[member] || found[guest] {
		t.Errorf("a inativar = %v; want o membro e nunca o avulso", found)
	}

	// Pagar a cobrança de um avulso inativo não o reativa como membro.
	if _, err := s.pool.Exec(ctx, `update users set status = 'inactive' where id = $1`, guest); err != nil {
		t.Fatal(err)
	}
	var chargeID string
	if err := s.pool.QueryRow(ctx, `select id from charges where user_id = $1`, guest).Scan(&chargeID); err != nil {
		t.Fatal(err)
	}
	if _, reactivated, err := s.MarkChargePaid(ctx, chargeID, "cash", admin); err != nil || reactivated {
		t.Errorf("avulso reativado = %v, %v", reactivated, err)
	}
}

// /admin/users mostra membros por padrão, inclusive na contagem.
func TestListUsersByKind(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	votingTestUser(t, s, "active")
	votingTestUser(t, s, "inactive")
	guest := isolationGuest(t, s)

	members, total, err := s.ListUsers(ctx, UserFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(members) != 2 {
		t.Errorf("padrão: %d de %d, want 2 membros", len(members), total)
	}
	for _, u := range members {
		if u.Kind != KindMember {
			t.Errorf("padrão listou %s (%s)", u.Name, u.Kind)
		}
	}
	explicit, total, err := s.ListUsers(ctx, UserFilter{Kind: KindMember, PerPage: 1})
	if err != nil || total != 2 || len(explicit) != 1 {
		t.Errorf("membros paginados: %d de %d, %v", len(explicit), total, err)
	}
	guests, total, err := s.ListUsers(ctx, UserFilter{Kind: KindGuest})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(guests) != 1 || guests[0].ID != guest || guests[0].Email != "" {
		t.Errorf("avulsos: %+v (total %d)", guests, total)
	}
	// Filtro de status combina com o tipo.
	if _, total, _ := s.ListUsers(ctx, UserFilter{Kind: KindGuest, Status: "inactive"}); total != 0 {
		t.Errorf("avulsos inativos = %d", total)
	}
}

// Avulso não é conta: nenhuma leitura por e-mail o encontra, nem se tiver
// um e-mail gravado, e leituras de usuário toleram e-mail e senha nulos.
func TestGuestIsNotAnAccount(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	guest := isolationGuest(t, s)
	u, err := s.UserByID(ctx, guest)
	if err != nil {
		t.Fatal(err)
	}
	if u.Kind != KindGuest || u.Email != "" {
		t.Errorf("UserByID do avulso = %+v", u)
	}

	var withEmail string
	if err := s.pool.QueryRow(ctx, `insert into users (name, email, password_hash, kind)
		values ('Avulso com e-mail', 'avulso@example.invalid', 'hash', 'guest') returning id`).Scan(&withEmail); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UserByEmail(ctx, "avulso@example.invalid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("login encontrou o avulso: %v", err)
	}
	if _, err := s.CreatePasswordReset(ctx, "avulso@example.invalid", "token", time.Now().Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("recuperação de senha para avulso: %v", err)
	}

	// O cadastro por convite sempre cria membro.
	member, err := s.CreateUser(ctx, "Membro", "membro@example.invalid", "", "hash", "#3B82A0", "player")
	if err != nil {
		t.Fatal(err)
	}
	if member.Kind != KindMember {
		t.Errorf("cadastro criou %q", member.Kind)
	}
	if _, hash, err := s.UserByEmail(ctx, "MEMBRO@example.invalid"); err != nil || hash != "hash" {
		t.Errorf("login do membro: %q, %v", hash, err)
	}
}

// Os checks da migration 015: membro continua exigindo credenciais e avulso
// nunca é admin.
func TestGuestConstraints(t *testing.T) {
	s := newTestStore(t, "isolation_test")
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `insert into users (name, kind) values ('Membro sem e-mail', 'member')`); err == nil {
		t.Error("membro sem e-mail e senha foi aceito")
	}
	if _, err := s.pool.Exec(ctx, `insert into users (name, kind, role) values ('Avulso admin', 'guest', 'admin')`); err == nil {
		t.Error("avulso admin foi aceito")
	}
	if _, err := s.pool.Exec(ctx, `insert into users (name, kind) values ('Outro', 'visitante')`); err == nil {
		t.Error("tipo desconhecido foi aceito")
	}
	var setting string
	if err := s.pool.QueryRow(ctx, `select value from settings where key = 'guest_price_cents'`).Scan(&setting); err != nil || setting != "2500" {
		t.Errorf("guest_price_cents = %q, %v", setting, err)
	}
}
