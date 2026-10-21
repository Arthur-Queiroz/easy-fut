package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"futdarapaziada/api/internal/auth"
	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/store"
)

const guestVoteAppURL = "https://fut.example.invalid"

// guestVoteFixture: partida encerrada (closed) com um admin e um membro que
// foram (going), um membro que não foi, um avulso confirmado e um com o Pix
// pendente.
type guestVoteFixture struct {
	s         *Server
	raw       *pgxpool.Pool
	matchID   string
	admin     string
	player    string
	absent    string
	guest     store.GuestBooking
	pendingID string
}

func newGuestVoteFixture(t *testing.T) guestVoteFixture {
	t.Helper()
	ctx := context.Background()
	s, raw := apiTestServer(t)
	s.cfg.AppURL = guestVoteAppURL
	f := guestVoteFixture{s: s, raw: raw}
	f.admin = insertUser(t, raw, "member", "admin", "admin@example.invalid", "senha-de-teste-1")
	f.player = insertUser(t, raw, "member", "player", "jogador@example.invalid", "senha-de-teste-1")
	f.absent = insertUser(t, raw, "member", "player", "ausente@example.invalid", "senha-de-teste-1")

	m, err := s.store.CreateMatch(ctx, store.MatchInput{
		MatchDate: "2099-01-10", StartTime: "20:00", EndTime: "22:00", Venue: "Arena Teste",
		ConfirmationDeadline: time.Now().Add(time.Hour),
	}, f.admin)
	if err != nil {
		t.Fatal(err)
	}
	f.matchID = m.ID
	for _, id := range []string{f.admin, f.player} {
		if err := s.store.Confirm(ctx, m.ID, id, "going"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.SetGuestSale(ctx, m.ID, true, 3, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	book := func(phone string) store.GuestBooking {
		t.Helper()
		b, err := s.store.CreateGuestBooking(ctx, store.GuestBookingInput{
			MatchID: m.ID, Name: "Avulso " + phone[len(phone)-1:], Phone: phone, Email: "avulso@example.invalid",
		}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		b, err = s.store.MarkGuestBookingPendingPayment(ctx, b.ID, store.GuestPix{OrderID: "order-" + b.ID})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	g := book("11987650001")
	if f.guest, _, err = s.store.ConfirmGuestPayment(ctx, g.ID, g.ProviderOrderID, "pay-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	f.pendingID = book("11987650002").ID
	if _, err := raw.Exec(ctx, `update matches set status = 'closed' where id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	return f
}

// finish abre a votação pela ação do admin.
func (f guestVoteFixture) finish(t *testing.T) {
	t.Helper()
	if rec := sessionRequest(t, f.s, "POST", "/api/matches/"+f.matchID+"/finish", "", f.admin); rec.Code != http.StatusOK {
		t.Fatalf("encerrar partida: %d %s", rec.Code, rec.Body)
	}
}

// voteLinks devolve as mensagens de link de voto da inscrição, na ordem.
func (f guestVoteFixture) voteLinks(t *testing.T, bookingID string) []string {
	t.Helper()
	rows, err := f.raw.Query(context.Background(), `
		select n.message from guest_messages gm join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'vote_link' order by gm.created_at`, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func (f guestVoteFixture) lastToken(t *testing.T) string {
	t.Helper()
	links := f.voteLinks(t, f.guest.ID)
	if len(links) == 0 {
		t.Fatal("nenhum link de voto agendado")
	}
	_, token, ok := strings.Cut(links[len(links)-1], guestVoteAppURL+"/votar/")
	if !ok || token == "" {
		t.Fatalf("mensagem sem link: %q", links[len(links)-1])
	}
	return token
}

func guestVoteRequest(s *Server, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body apiError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("corpo de erro: %v", err)
	}
	return body.Error
}

// Encerrar a partida agenda um link por avulso confirmado (nenhum para o
// pendente) e não reenvia se a ação se repetir.
func TestFinishMatchSchedulesGuestVoteLinks(t *testing.T) {
	f := newGuestVoteFixture(t)
	f.finish(t)
	if rec := sessionRequest(t, f.s, "POST", "/api/matches/"+f.matchID+"/finish", "", f.admin); rec.Code == http.StatusOK {
		t.Errorf("encerrar de novo: %d", rec.Code)
	}

	links := f.voteLinks(t, f.guest.ID)
	if len(links) != 1 || !strings.HasPrefix(links[0], "A votação da partida de 10/01/2099 está aberta até ") {
		t.Fatalf("links = %q", links)
	}
	if got := f.voteLinks(t, f.pendingID); len(got) != 0 {
		t.Errorf("pendente recebeu link: %q", got)
	}
	var phone string
	if err := f.raw.QueryRow(context.Background(), `select n.phone from guest_messages gm
		join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'vote_link'`, f.guest.ID).Scan(&phone); err != nil || phone != "5511987650001" {
		t.Errorf("telefone = %q, %v", phone, err)
	}
}

type guestVoteView struct {
	Match        store.Match         `json:"match"`
	Participants []map[string]string `json:"participants"`
	MyVotes      map[string][]string `json:"my_votes"`
	ClosesAt     *time.Time          `json:"voting_closes_at"`
}

func TestGuestVoteGet(t *testing.T) {
	f := newGuestVoteFixture(t)
	f.finish(t)
	token := f.lastToken(t)

	rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body)
	}
	var view guestVoteView
	if err := json.NewDecoder(rec.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if view.Match.ID != f.matchID || view.Match.Status != "voting" || view.ClosesAt == nil {
		t.Errorf("partida = %+v, prazo %v", view.Match, view.ClosesAt)
	}
	if view.MyVotes == nil || len(view.MyVotes) != 0 {
		t.Errorf("my_votes = %v, want {}", view.MyVotes)
	}
	ids := map[string]bool{}
	for _, p := range view.Participants {
		ids[p["user_id"]] = true
		if p["name"] == "" || p["avatar_color"] == "" {
			t.Errorf("participante incompleto: %v", p)
		}
		if len(p) != 3 {
			t.Errorf("participante expõe mais do que o contrato: %v", p)
		}
	}
	if len(ids) != 3 || !ids[f.admin] || !ids[f.player] || !ids[f.guest.UserID] || ids[f.absent] {
		t.Errorf("participantes = %v", view.Participants)
	}

	if rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/token-que-nao-existe", ""); rec.Code != http.StatusNotFound {
		t.Errorf("token inválido: %d", rec.Code)
	}

	// Prazo do link vencido antes do job fechar a votação: 410.
	if _, err := f.raw.Exec(context.Background(), `update guest_vote_tokens set expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+token, ""); rec.Code != http.StatusGone {
		t.Errorf("link vencido: %d", rec.Code)
	}
	if _, err := f.raw.Exec(context.Background(), `update guest_vote_tokens set expires_at = now() + interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	// Votação encerrada pelo admin: 410.
	if rec := sessionRequest(t, f.s, "POST", "/api/matches/"+f.matchID+"/close-voting", "", f.admin); rec.Code != http.StatusOK {
		t.Fatalf("fechar votação: %d", rec.Code)
	}
	if rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+token, ""); rec.Code != http.StatusGone {
		t.Errorf("votação encerrada: %d", rec.Code)
	}
}

// O avulso vota com as mesmas regras e mensagens dos membros.
func TestGuestVotePut(t *testing.T) {
	f := newGuestVoteFixture(t)
	f.finish(t)
	token := f.lastToken(t)
	put := func(body string) *httptest.ResponseRecorder {
		return guestVoteRequest(f.s, "PUT", "/api/public/guest-vote/"+token+"/votes", body)
	}
	votes := func(ids ...string) string {
		b, _ := json.Marshal(ids)
		return string(b)
	}

	rec := put(`{"votes":{"top_scorer":` + votes(f.player, f.admin) + `,"worst_player":` + votes(f.player) + `}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("voto válido: %d %s", rec.Code, rec.Body)
	}
	rec = guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+token, "")
	var view guestVoteView
	if err := json.NewDecoder(rec.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if got := view.MyVotes["top_scorer"]; len(got) != 2 || got[0] != f.player || got[1] != f.admin {
		t.Errorf("my_votes = %v", view.MyVotes)
	}
	var voter string
	if err := f.raw.QueryRow(context.Background(), `select distinct voter_id::text from votes where match_id = $1`, f.matchID).Scan(&voter); err != nil || voter != f.guest.UserID {
		t.Errorf("eleitor gravado = %q, %v", voter, err)
	}
	// Membros também podem votar no avulso.
	if rec := sessionRequest(t, f.s, "PUT", "/api/matches/"+f.matchID+"/votes",
		`{"votes":{"best_goal":`+votes(f.guest.UserID)+`}}`, f.player); rec.Code != http.StatusOK {
		t.Errorf("membro vota no avulso: %d %s", rec.Code, rec.Body)
	}

	cases := []struct {
		name   string
		body   string
		status int
		msg    string
	}{
		{"três votos na categoria", `{"votes":{"top_scorer":` + votes(f.player, f.admin, f.guest.UserID) + `}}`, http.StatusConflict, "você já usou seus 2 votos nesta categoria"},
		{"perna de pau em si mesmo", `{"votes":{"worst_player":` + votes(f.guest.UserID) + `}}`, http.StatusBadRequest, "não vale votar em si mesmo nessa categoria"},
		{"candidato que não foi", `{"votes":{"top_scorer":` + votes(f.absent) + `}}`, http.StatusForbidden, "somente quem participou da partida pode votar e receber votos"},
		{"categoria inválida", `{"votes":{"goleiro":` + votes(f.player) + `}}`, http.StatusBadRequest, "categoria inválida"},
		{"candidato malformado", `{"votes":{"top_scorer":["x"]}}`, http.StatusBadRequest, "candidate_id inválido"},
		{"corpo inválido", `{`, http.StatusBadRequest, "corpo da requisição inválido"},
	}
	for _, c := range cases {
		rec := put(c.body)
		if rec.Code != c.status || errorMessage(t, rec) != c.msg {
			t.Errorf("%s: %d, want %d %q", c.name, rec.Code, c.status, c.msg)
		}
	}

	if rec := guestVoteRequest(f.s, "PUT", "/api/public/guest-vote/token-que-nao-existe/votes", `{"votes":{}}`); rec.Code != http.StatusNotFound {
		t.Errorf("token inválido: %d", rec.Code)
	}

	// O admin corrige a presença: quem não foi não vota.
	if _, err := f.raw.Exec(context.Background(), `update match_confirmations set response = 'not_going'
		where match_id = $1 and user_id = $2`, f.matchID, f.guest.UserID); err != nil {
		t.Fatal(err)
	}
	if rec := put(`{"votes":{"top_scorer":[]}}`); rec.Code != http.StatusForbidden {
		t.Errorf("avulso fora da presença: %d", rec.Code)
	}

	if rec := sessionRequest(t, f.s, "POST", "/api/matches/"+f.matchID+"/close-voting", "", f.admin); rec.Code != http.StatusOK {
		t.Fatalf("fechar votação: %d", rec.Code)
	}
	if rec := put(`{"votes":{"top_scorer":[]}}`); rec.Code != http.StatusGone {
		t.Errorf("votação encerrada: %d", rec.Code)
	}
}

// Limite por IP nos endpoints públicos de voto.
func TestGuestVoteRateLimit(t *testing.T) {
	f := newGuestVoteFixture(t)
	f.s.guestVoteLimiter = auth.NewLoginLimiter(2, time.Minute)
	request := func(ip string) int {
		req := httptest.NewRequest("GET", "/api/public/guest-vote/token-que-nao-existe", nil)
		if ip != "" {
			req.Header.Set("CF-Connecting-IP", ip)
		}
		rec := httptest.NewRecorder()
		f.s.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	for range 2 {
		if code := request("203.0.113.7"); code != http.StatusNotFound {
			t.Fatalf("dentro do limite: %d", code)
		}
	}
	if code := request("203.0.113.7"); code != http.StatusTooManyRequests {
		t.Errorf("acima do limite: %d, want 429", code)
	}
	if code := request("203.0.113.8"); code != http.StatusNotFound {
		t.Errorf("outro IP: %d", code)
	}
	// O PUT conta no mesmo limite.
	req := httptest.NewRequest("PUT", "/api/public/guest-vote/x/votes", strings.NewReader(`{"votes":{}}`))
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	rec := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("PUT acima do limite: %d", rec.Code)
	}
}

func TestResendGuestBookingMessage(t *testing.T) {
	f := newGuestVoteFixture(t)
	resend := func(bookingID, kind, userID string) *httptest.ResponseRecorder {
		return sessionRequest(t, f.s, "POST", "/api/admin/guest-bookings/"+bookingID+"/resend", `{"kind":"`+kind+`"}`, userID)
	}

	if rec := resend(f.guest.ID, "confirmation", f.player); rec.Code != http.StatusForbidden {
		t.Errorf("não-admin: %d", rec.Code)
	}
	if rec := resend(f.guest.ID, "confirmation", f.admin); rec.Code != http.StatusOK {
		t.Fatalf("reenviar confirmação: %d %s", rec.Code, rec.Body)
	}
	var resent int
	if err := f.raw.QueryRow(context.Background(), `select count(*) from guest_messages gm
		join notifications n on n.id = gm.notification_id
		where gm.booking_id = $1 and gm.kind = 'confirmation' and gm.resend
		  and n.message = $2`, f.guest.ID, guests.ConfirmationMessage("2099-01-10", "20:00", "Arena Teste")).Scan(&resent); err != nil || resent != 1 {
		t.Errorf("confirmação reenviada: %d, %v", resent, err)
	}

	rec := resend(f.guest.ID, "vote_link", f.admin)
	if rec.Code != http.StatusConflict || errorMessage(t, rec) != guests.ErrVoteLinkVotingClosed.Error() {
		t.Errorf("link antes da votação: %d", rec.Code)
	}
	rec = resend(f.pendingID, "confirmation", f.admin)
	if rec.Code != http.StatusConflict || errorMessage(t, rec) != guests.ErrResendNotConfirmed.Error() {
		t.Errorf("confirmação de pendente: %d", rec.Code)
	}
	if rec := resend(f.guest.ID, "pix", f.admin); rec.Code != http.StatusBadRequest {
		t.Errorf("tipo inválido: %d", rec.Code)
	}
	if rec := resend("00000000-0000-0000-0000-000000000000", "confirmation", f.admin); rec.Code != http.StatusNotFound {
		t.Errorf("inscrição inexistente: %d", rec.Code)
	}
	if rec := resend("nao-e-uuid", "confirmation", f.admin); rec.Code != http.StatusNotFound {
		t.Errorf("id malformado: %d", rec.Code)
	}

	f.finish(t)
	oldToken := f.lastToken(t)
	if rec := resend(f.guest.ID, "vote_link", f.admin); rec.Code != http.StatusOK {
		t.Fatalf("reenviar link: %d %s", rec.Code, rec.Body)
	}
	newToken := f.lastToken(t)
	if newToken == oldToken || len(f.voteLinks(t, f.guest.ID)) != 2 {
		t.Fatal("o reenvio não gerou um link novo")
	}
	if rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+oldToken, ""); rec.Code != http.StatusNotFound {
		t.Errorf("token antigo: %d, want 404", rec.Code)
	}
	if rec := guestVoteRequest(f.s, "GET", "/api/public/guest-vote/"+newToken, ""); rec.Code != http.StatusOK {
		t.Errorf("token novo: %d", rec.Code)
	}
}

// Selo Avulso: a partida expõe kind nas confirmações e nos times.
func TestMatchExposesGuestKind(t *testing.T) {
	f := newGuestVoteFixture(t)
	rec := sessionRequest(t, f.s, "POST", "/api/matches/"+f.matchID+"/draw-teams", `{"team_count":2}`, f.admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("sorteio: %d %s", rec.Code, rec.Body)
	}
	rec = sessionRequest(t, f.s, "GET", "/api/matches/"+f.matchID, "", f.player)
	var body struct {
		Confirmations []map[string]any `json:"confirmations"`
		Teams         []struct {
			Members []map[string]any `json:"members"`
		} `json:"teams"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	kindOf := func(entries []map[string]any) map[string]any {
		out := map[string]any{}
		for _, e := range entries {
			out[e["user_id"].(string)] = e["kind"]
		}
		return out
	}
	confirmations := kindOf(body.Confirmations)
	if confirmations[f.guest.UserID] != "guest" || confirmations[f.player] != "member" {
		t.Errorf("confirmações: %v", confirmations)
	}
	var members []map[string]any
	for _, team := range body.Teams {
		members = append(members, team.Members...)
	}
	teams := kindOf(members)
	if len(teams) != 3 || teams[f.guest.UserID] != "guest" || teams[f.admin] != "member" {
		t.Errorf("times: %v", teams)
	}
}

// Contrato dos erros do reenvio.
func TestWriteStoreErrorGuestResendContract(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{guests.ErrInvalidMessageKind, http.StatusBadRequest},
		{guests.ErrResendNotConfirmed, http.StatusConflict},
		{guests.ErrVoteLinkVotingClosed, http.StatusConflict},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeStoreError(rec, tc.err)
		if rec.Code != tc.status || errorMessage(t, rec) != tc.err.Error() {
			t.Errorf("%v: %d, want %d", tc.err, rec.Code, tc.status)
		}
	}
}
