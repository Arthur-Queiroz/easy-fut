package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/store"
)

// Participação avulsa — admin (fase B): venda por partida, inscrições,
// reembolso e caixa.

func guestAdminFixture(t *testing.T) (*Server, *pgxpool.Pool, string, string) {
	t.Helper()
	s, raw := apiTestServer(t)
	admin := insertUser(t, raw, "member", "admin", "admin@example.invalid", "senha-de-teste-1")
	player := insertUser(t, raw, "member", "player", "jogador@example.invalid", "senha-de-teste-1")
	return s, raw, admin, player
}

func guestAdminMatch(t *testing.T, s *Server, admin, date string) store.Match {
	t.Helper()
	m, err := s.store.CreateMatch(context.Background(), store.MatchInput{
		MatchDate: date, StartTime: "20:00", EndTime: "22:00",
		Venue: "Arena " + date, Address: "Rua Teste, 1",
		ConfirmationDeadline: time.Now().Add(2 * time.Hour),
	}, admin)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func decodeMatch(t *testing.T, rec interface{ Bytes() []byte }) store.Match {
	t.Helper()
	var m store.Match
	if err := json.Unmarshal(rec.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// book cria uma inscrição e a leva até o estado pedido: reserved,
// pending_payment ou confirmed (pagamento aprovado pelo webhook).
func book(t *testing.T, s *Server, matchID, name, phone, until string) store.GuestBooking {
	t.Helper()
	ctx := context.Background()
	b, err := s.store.CreateGuestBooking(ctx, store.GuestBookingInput{
		MatchID: matchID, Name: name, Phone: phone, Email: "avulso@example.invalid",
		AccessTokenHash: "hash-" + uuid.NewString(),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if until == "reserved" {
		return b
	}
	b, err = s.store.MarkGuestBookingPendingPayment(ctx, b.ID, store.GuestPix{OrderID: "order-" + b.ID, Status: "action_required"})
	if err != nil {
		t.Fatal(err)
	}
	if until == "pending_payment" {
		return b
	}
	b, _, err = s.store.ConfirmGuestPayment(ctx, b.ID, b.ProviderOrderID, "pay-"+b.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func putSetting(t *testing.T, s *Server, admin, key, value string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{key: value})
	if rec := sessionRequest(t, s, "PUT", "/api/admin/settings", string(body), admin); rec.Code != http.StatusOK {
		t.Fatalf("configurar %s: %d %s", key, rec.Code, rec.Body)
	}
}

func TestGuestSaleOpenAndClose(t *testing.T) {
	s, _, admin, player := guestAdminFixture(t)
	m := guestAdminMatch(t, s, admin, "2099-01-10")
	path := "/api/admin/matches/" + m.ID + "/guest-sale"

	// Preço e taxa vêm das configurações vigentes (pix_fee_bps é a mesma
	// taxa repassada nas mensalidades).
	putSetting(t, s, admin, "guest_price_cents", "3000")
	putSetting(t, s, admin, "pix_fee_bps", "199")
	want := billing.FeeRate{BasisPoints: 199}.PassOn(3000)

	if rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":3}`, player); rec.Code != http.StatusForbidden {
		t.Errorf("jogador abrindo venda: %d, want 403", rec.Code)
	}

	rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":3}`, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("abrir venda: %d %s", rec.Code, rec.Body)
	}
	got := decodeMatch(t, rec.Body)
	if !got.GuestSaleOpen || got.GuestSlots != 3 || got.GuestPriceCents != 3000 || got.GuestTotalCents != want.TotalCents {
		t.Fatalf("venda aberta = %+v, want 3 vagas a %d/%d", got, 3000, want.TotalCents)
	}

	// Reajustar as vagas com a venda aberta não mexe no preço congelado.
	putSetting(t, s, admin, "guest_price_cents", "4000")
	got = decodeMatch(t, sessionRequest(t, s, "PUT", path, `{"open":true,"slots":5}`, admin).Body)
	if got.GuestSlots != 5 || got.GuestPriceCents != 3000 || got.GuestTotalCents != want.TotalCents {
		t.Errorf("reajuste de vagas = %+v", got)
	}

	// Fechar mantém os valores congelados.
	rec = sessionRequest(t, s, "PUT", path, `{"open":false,"slots":5}`, admin)
	got = decodeMatch(t, rec.Body)
	if rec.Code != http.StatusOK || got.GuestSaleOpen || got.GuestSlots != 5 || got.GuestPriceCents != 3000 {
		t.Errorf("fechar venda: %d %+v", rec.Code, got)
	}

	// Reabrir congela o preço vigente.
	got = decodeMatch(t, sessionRequest(t, s, "PUT", path, `{"open":true,"slots":5}`, admin).Body)
	if !got.GuestSaleOpen || got.GuestPriceCents != 4000 || got.GuestTotalCents != (billing.FeeRate{BasisPoints: 199}).PassOn(4000).TotalCents {
		t.Errorf("reabrir = %+v", got)
	}

	for _, body := range []string{`{"open":true,"slots":-1}`, `{"open":true,"slots":101}`, `{"open":true,"slots":"x"}`, `x`} {
		if rec := sessionRequest(t, s, "PUT", path, body, admin); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", body, rec.Code)
		}
	}
	for _, id := range []string{"nao-e-uuid", uuid.NewString()} {
		if rec := sessionRequest(t, s, "PUT", "/api/admin/matches/"+id+"/guest-sale", `{"open":true,"slots":1}`, admin); rec.Code != http.StatusNotFound {
			t.Errorf("partida %s: %d, want 404", id, rec.Code)
		}
	}
}

func TestGuestSaleSlotsAndMatchStatus(t *testing.T) {
	s, _, admin, _ := guestAdminFixture(t)
	m := guestAdminMatch(t, s, admin, "2099-01-10")
	path := "/api/admin/matches/" + m.ID + "/guest-sale"
	if rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":3}`, admin); rec.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", rec.Code, rec.Body)
	}
	book(t, s, m.ID, "Ana Avulsa", "11987654321", "confirmed")
	book(t, s, m.ID, "Bia Avulsa", "11987654322", "pending_payment")

	// Duas inscrições ocupam vaga: não dá para deixar só uma.
	rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":1}`, admin)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "inscrições") {
		t.Errorf("vagas abaixo das inscrições: %d %s", rec.Code, rec.Body)
	}
	if rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":2}`, admin); rec.Code != http.StatusOK {
		t.Errorf("vagas iguais às inscrições: %d %s", rec.Code, rec.Body)
	}

	// Partida que não está aberta não abre venda, mas pode fechar.
	if err := s.store.SetMatchStatus(context.Background(), m.ID, "closed", "open"); err != nil {
		t.Fatal(err)
	}
	if rec := sessionRequest(t, s, "PUT", path, `{"open":true,"slots":4}`, admin); rec.Code != http.StatusConflict {
		t.Errorf("abrir com a partida fechada: %d, want 409", rec.Code)
	}
	if rec := sessionRequest(t, s, "PUT", path, `{"open":false,"slots":4}`, admin); rec.Code != http.StatusOK {
		t.Errorf("fechar com a partida fechada: %d %s", rec.Code, rec.Body)
	}
}

// Toda resposta com partida traz a venda e as contagens de avulsos.
func TestMatchGuestFields(t *testing.T) {
	s, raw, admin, _ := guestAdminFixture(t)
	m := guestAdminMatch(t, s, admin, "2099-01-10")
	if rec := sessionRequest(t, s, "PUT", "/api/admin/matches/"+m.ID+"/guest-sale", `{"open":true,"slots":6}`, admin); rec.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", rec.Code, rec.Body)
	}
	book(t, s, m.ID, "Ana Avulsa", "11987654321", "confirmed")
	book(t, s, m.ID, "Bia Avulsa", "11987654322", "confirmed")
	book(t, s, m.ID, "Caio Avulso", "11987654323", "pending_payment")
	book(t, s, m.ID, "Duda Avulsa", "11987654324", "reserved")
	expired := book(t, s, m.ID, "Edu Avulso", "11987654325", "reserved")
	if _, err := raw.Exec(context.Background(), `update guest_bookings set status = 'expired' where id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}

	rec := sessionRequest(t, s, "GET", "/api/matches", "", admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("listar partidas: %d %s", rec.Code, rec.Body)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("partidas: %v %s", err, rec.Body)
	}
	want := map[string]any{
		"guest_sale_open": true, "guest_slots": 6.0, "guest_price_cents": 2500.0, "guest_total_cents": 2525.0,
		"guest_confirmed_count": 2.0, "guest_pending_count": 2.0,
	}
	for key, value := range want {
		if list[0][key] != value {
			t.Errorf("%s = %v, want %v", key, list[0][key], value)
		}
	}
	// Os confirmados entram na presença como going.
	if list[0]["going_count"] != 2.0 {
		t.Errorf("going_count = %v", list[0]["going_count"])
	}
}

func TestAdminGuestBookingsList(t *testing.T) {
	s, raw, admin, player := guestAdminFixture(t)
	ctx := context.Background()
	m1 := guestAdminMatch(t, s, admin, "2099-01-10")
	m2 := guestAdminMatch(t, s, admin, "2099-01-17")
	for _, m := range []store.Match{m1, m2} {
		if rec := sessionRequest(t, s, "PUT", "/api/admin/matches/"+m.ID+"/guest-sale", `{"open":true,"slots":5}`, admin); rec.Code != http.StatusOK {
			t.Fatalf("abrir: %d %s", rec.Code, rec.Body)
		}
	}
	ana := book(t, s, m1.ID, "Ana Souza", "(11) 98765-4321", "confirmed")
	bia := book(t, s, m1.ID, "Bia Lima", "21 99876-5432", "pending_payment")
	ana2 := book(t, s, m2.ID, "Ana Souza", "11987654321", "reserved")

	// O WhatsApp de confirmação da Ana foi entregue pela fila.
	sentAt := time.Date(2099, 1, 5, 12, 0, 0, 0, time.UTC)
	if _, err := raw.Exec(ctx, `
		update notifications n set status = 'sent', sent_at = $2
		from guest_messages gm where gm.notification_id = n.id and gm.booking_id = $1`, ana.ID, sentAt); err != nil {
		t.Fatal(err)
	}

	list := func(query string) (int, []map[string]any) {
		t.Helper()
		rec := sessionRequest(t, s, "GET", "/api/admin/guest-bookings"+query, "", admin)
		var items []map[string]any
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
				t.Fatalf("%s: %v %s", query, err, rec.Body)
			}
		}
		return rec.Code, items
	}
	ids := func(items []map[string]any) []string {
		var out []string
		for _, it := range items {
			out = append(out, it["id"].(string))
		}
		return out
	}

	code, items := list("")
	if code != http.StatusOK || !slices.Equal(ids(items), []string{ana2.ID, bia.ID, ana.ID}) {
		t.Fatalf("lista completa (mais recente primeiro): %d %v", code, ids(items))
	}

	// JSON exatamente o do contrato.
	keys := make([]string, 0, len(items[2]))
	for k := range items[2] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	wantKeys := []string{"base_cents", "confirmation_sent_at", "confirmed_at", "created_at", "email", "id",
		"match_date", "match_id", "name", "phone", "refund_reason", "start_time", "status", "total_cents",
		"venue", "vote_link_sent_at"}
	if !slices.Equal(keys, wantKeys) {
		t.Errorf("campos = %v, want %v", keys, wantKeys)
	}
	first := items[2]
	if first["match_id"] != m1.ID || first["match_date"] != "2099-01-10" || first["start_time"] != "20:00" ||
		first["venue"] != "Arena 2099-01-10" || first["phone"] != "+5511987654321" || first["status"] != "confirmed" ||
		first["base_cents"] != 2500.0 || first["total_cents"] != 2525.0 || first["confirmed_at"] == nil ||
		first["vote_link_sent_at"] != nil {
		t.Errorf("inscrição confirmada = %v", first)
	}
	if sent, _ := time.Parse(time.RFC3339, first["confirmation_sent_at"].(string)); !sent.Equal(sentAt) {
		t.Errorf("confirmation_sent_at = %v, want %v", first["confirmation_sent_at"], sentAt)
	}
	if items[1]["confirmation_sent_at"] != nil || items[1]["confirmed_at"] != nil {
		t.Errorf("pendente com envio ou confirmação: %v", items[1])
	}

	cases := []struct {
		query string
		want  []string
	}{
		{"?match_id=" + m1.ID, []string{bia.ID, ana.ID}},
		{"?status=confirmed", []string{ana.ID}},
		{"?match_id=" + m2.ID + "&status=reserved", []string{ana2.ID}},
		{"?q=" + url.QueryEscape("ana sou"), []string{ana2.ID, ana.ID}},
		{"?q=" + url.QueryEscape("+55 (21) 99876-5432"), []string{bia.ID}},
		{"?q=" + url.QueryEscape("11 98765 4321"), []string{ana2.ID, ana.ID}},
		{"?q=" + url.QueryEscape("99876"), []string{bia.ID}},
		{"?q=" + url.QueryEscape("%"), nil},
		{"?q=" + url.QueryEscape("ninguém"), nil},
	}
	for _, tc := range cases {
		code, items := list(tc.query)
		if code != http.StatusOK || !slices.Equal(ids(items), tc.want) {
			t.Errorf("%s: %d %v, want %v", tc.query, code, ids(items), tc.want)
		}
		if code == http.StatusOK && items == nil {
			t.Errorf("%s: lista vazia veio null", tc.query)
		}
	}

	for _, query := range []string{"?status=pago", "?match_id=abc"} {
		if code, _ := list(query); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", query, code)
		}
	}
	if rec := sessionRequest(t, s, "GET", "/api/admin/guest-bookings", "", player); rec.Code != http.StatusForbidden {
		t.Errorf("jogador na lista: %d, want 403", rec.Code)
	}
}

func TestAdminGuestRefund(t *testing.T) {
	s, raw, admin, player := guestAdminFixture(t)
	ctx := context.Background()
	m := guestAdminMatch(t, s, admin, "2099-01-10")
	other := guestAdminMatch(t, s, admin, "2099-01-17")
	for _, match := range []store.Match{m, other} {
		if rec := sessionRequest(t, s, "PUT", "/api/admin/matches/"+match.ID+"/guest-sale", `{"open":true,"slots":5}`, admin); rec.Code != http.StatusOK {
			t.Fatalf("abrir: %d %s", rec.Code, rec.Body)
		}
	}
	paid := book(t, s, m.ID, "Ana Souza", "11987654321", "confirmed")
	stillPlaying := book(t, s, other.ID, "Bia Lima", "11987654322", "confirmed")

	if rec := sessionRequest(t, s, "POST", "/api/matches/"+m.ID+"/cancel", `{"reason":"chuva"}`, admin); rec.Code != http.StatusOK {
		t.Fatalf("cancelar partida: %d %s", rec.Code, rec.Body)
	}
	path := "/api/admin/guest-bookings/" + paid.ID + "/refunded"

	if rec := sessionRequest(t, s, "POST", path, "", player); rec.Code != http.StatusForbidden {
		t.Errorf("jogador marcando estorno: %d, want 403", rec.Code)
	}

	for range 2 { // a segunda vez é repetição: nada muda
		rec := sessionRequest(t, s, "POST", path, "", admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("marcar estorno: %d %s", rec.Code, rec.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["id"] != paid.ID || body["status"] != "refunded" || body["refund_reason"] == "" {
			t.Errorf("inscrição estornada = %v", body)
		}
	}

	var refunds int
	var amount, fee int64
	var createdBy *string
	if err := raw.QueryRow(ctx, `select count(*), coalesce(sum(amount_cents), 0), coalesce(sum(fee_cents), 0), max(created_by::text)
		from cash_entries where kind = 'refund' and booking_id = $1`, paid.ID).Scan(&refunds, &amount, &fee, &createdBy); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 || amount != paid.BaseCents || fee != paid.FeeCents || createdBy == nil || *createdBy != admin {
		t.Errorf("saída no caixa: %d lançamentos, %d/%d, por %v", refunds, amount, fee, createdBy)
	}
	var logged int
	if err := raw.QueryRow(ctx, `select count(*) from activity_log where kind = 'guest_refunded'`).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 1 {
		t.Errorf("atividades de estorno = %d, want 1", logged)
	}

	// O saldo volta a refletir só a inscrição que segue valendo.
	rec := sessionRequest(t, s, "GET", "/api/admin/cash", "", admin)
	var report struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil || report.BalanceCents != stillPlaying.BaseCents {
		t.Errorf("saldo = %d (%v), want %d", report.BalanceCents, err, stillPlaying.BaseCents)
	}

	// Só de refund_pending: uma inscrição confirmada numa partida em pé, não.
	if rec := sessionRequest(t, s, "POST", "/api/admin/guest-bookings/"+stillPlaying.ID+"/refunded", "", admin); rec.Code != http.StatusConflict {
		t.Errorf("estorno de confirmada: %d, want 409", rec.Code)
	}
	for _, id := range []string{"abc", uuid.NewString()} {
		if rec := sessionRequest(t, s, "POST", "/api/admin/guest-bookings/"+id+"/refunded", "", admin); rec.Code != http.StatusNotFound {
			t.Errorf("inscrição %s: %d, want 404", id, rec.Code)
		}
	}

	// Sem entrada no caixa (dinheiro que não passou por ele), não há saída.
	unpaid := book(t, s, other.ID, "Caio Lima", "11987654323", "reserved")
	if _, err := raw.Exec(ctx, `update guest_bookings set status = 'refund_pending' where id = $1`, unpaid.ID); err != nil {
		t.Fatal(err)
	}
	if rec := sessionRequest(t, s, "POST", "/api/admin/guest-bookings/"+unpaid.ID+"/refunded", "", admin); rec.Code != http.StatusOK {
		t.Errorf("estorno sem entrada: %d %s", rec.Code, rec.Body)
	}
	if err := raw.QueryRow(ctx, `select count(*) from cash_entries where booking_id = $1`, unpaid.ID).Scan(&refunds); err != nil || refunds != 0 {
		t.Errorf("lançamentos sem entrada = %d (%v)", refunds, err)
	}
}

func TestAdminCash(t *testing.T) {
	s, _, admin, player := guestAdminFixture(t)
	m := guestAdminMatch(t, s, admin, "2099-01-10")
	if rec := sessionRequest(t, s, "PUT", "/api/admin/matches/"+m.ID+"/guest-sale", `{"open":true,"slots":5}`, admin); rec.Code != http.StatusOK {
		t.Fatalf("abrir: %d %s", rec.Code, rec.Body)
	}
	paid := book(t, s, m.ID, "Ana Souza", "11987654321", "confirmed")

	post := func(body string) (int, map[string]any) {
		t.Helper()
		rec := sessionRequest(t, s, "POST", "/api/admin/cash/expenses", body, admin)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	for _, body := range []string{
		`{"description":"Bola","amount_cents":900,"occurred_on":"2099-13-01"}`,
		`{"description":"Bola","amount_cents":900,"occurred_on":"05/01/2099"}`,
		`{"description":"Bola","amount_cents":900}`,
		`{"description":" ","amount_cents":900,"occurred_on":"2099-01-05"}`,
		`{"description":"Bola","amount_cents":0,"occurred_on":"2099-01-05"}`,
		`x`,
	} {
		if code, _ := post(body); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", body, code)
		}
	}
	if rec := sessionRequest(t, s, "POST", "/api/admin/cash/expenses", `{"description":"Bola","amount_cents":900,"occurred_on":"2099-01-05"}`, player); rec.Code != http.StatusForbidden {
		t.Errorf("jogador lançando despesa: %d, want 403", rec.Code)
	}

	code, ball := post(`{"description":" Bola nova ","amount_cents":900,"occurred_on":"2099-01-05"}`)
	if code != http.StatusCreated || ball["kind"] != "expense" || ball["description"] != "Bola nova" ||
		ball["amount_cents"] != 900.0 || ball["occurred_on"] != "2099-01-05" {
		t.Fatalf("despesa: %d %v", code, ball)
	}
	if code, _ := post(`{"description":"Coletes","amount_cents":400,"occurred_on":"2000-01-01"}`); code != http.StatusCreated {
		t.Fatalf("despesa antiga: %d", code)
	}

	cash := func(query string) (int, map[string]any) {
		t.Helper()
		rec := sessionRequest(t, s, "GET", "/api/admin/cash"+query, "", admin)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, report := cash("")
	entries, _ := report["entries"].([]any)
	if code != http.StatusOK || report["balance_cents"] != float64(paid.BaseCents-900-400) || len(entries) != 3 {
		t.Fatalf("caixa: %d %v", code, report)
	}
	for _, e := range entries {
		entry := e.(map[string]any)
		for _, key := range []string{"id", "kind", "amount_cents", "fee_cents", "booking_id", "description", "occurred_on", "created_at"} {
			if _, ok := entry[key]; !ok {
				t.Errorf("lançamento sem %s: %v", key, entry)
			}
		}
		if entry["kind"] == "guest_payment" && (entry["booking_id"] != paid.ID || entry["fee_cents"] != float64(paid.FeeCents)) {
			t.Errorf("entrada = %v", entry)
		}
	}

	// O período filtra a lista; o saldo é sempre o do caixa inteiro.
	code, report = cash("?from=2099-01-01&to=2099-01-31")
	entries, _ = report["entries"].([]any)
	if code != http.StatusOK || len(entries) != 1 || entries[0].(map[string]any)["id"] != ball["id"] ||
		report["balance_cents"] != float64(paid.BaseCents-900-400) {
		t.Errorf("período: %d %v", code, report)
	}
	code, report = cash("?from=1990-01-01&to=1990-01-31")
	if entries, ok := report["entries"].([]any); code != http.StatusOK || !ok || len(entries) != 0 {
		t.Errorf("período vazio deve vir como lista vazia: %d %v", code, report)
	}
	for _, query := range []string{"?from=2099-1-1", "?to=ontem", "?from=2099-02-01&to=2099-01-01"} {
		if code, _ := cash(query); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", query, code)
		}
	}
	if rec := sessionRequest(t, s, "GET", "/api/admin/cash", "", player); rec.Code != http.StatusForbidden {
		t.Errorf("jogador no caixa: %d, want 403", rec.Code)
	}

	// Só despesa sai; a entrada de uma inscrição, não.
	var paymentID string
	_, report = cash("")
	for _, e := range report["entries"].([]any) {
		if entry := e.(map[string]any); entry["kind"] == "guest_payment" {
			paymentID = entry["id"].(string)
		}
	}
	if rec := sessionRequest(t, s, "DELETE", "/api/admin/cash/expenses/"+paymentID, "", admin); rec.Code != http.StatusNotFound {
		t.Errorf("remover entrada: %d, want 404", rec.Code)
	}
	if rec := sessionRequest(t, s, "DELETE", "/api/admin/cash/expenses/"+ball["id"].(string), "", admin); rec.Code != http.StatusOK {
		t.Errorf("remover despesa: %d %s", rec.Code, rec.Body)
	}
	if rec := sessionRequest(t, s, "DELETE", "/api/admin/cash/expenses/"+ball["id"].(string), "", admin); rec.Code != http.StatusNotFound {
		t.Errorf("remover de novo: %d, want 404", rec.Code)
	}
	if _, report = cash(""); report["balance_cents"] != float64(paid.BaseCents-400) {
		t.Errorf("saldo depois de remover = %v", report["balance_cents"])
	}
}

func TestSettingsGuestPriceValidation(t *testing.T) {
	s, _, admin, _ := guestAdminFixture(t)
	for _, value := range []string{"99", "100001", "abc", "25,00", ""} {
		body, _ := json.Marshal(map[string]string{"guest_price_cents": value})
		if rec := sessionRequest(t, s, "PUT", "/api/admin/settings", string(body), admin); rec.Code != http.StatusBadRequest {
			t.Errorf("guest_price_cents=%q: %d, want 400", value, rec.Code)
		}
	}
	putSetting(t, s, admin, "guest_price_cents", "3000")
	if got := s.store.SettingString(context.Background(), "guest_price_cents", ""); got != "3000" {
		t.Errorf("guest_price_cents = %q", got)
	}
}
