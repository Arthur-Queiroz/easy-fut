package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"futdarapaziada/api/internal/billing"
	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/mercadopago"
	"futdarapaziada/api/internal/store"
)

const testWebhookSecret = "webhook-secret-de-teste"

// fakeMercadoPago faz o papel da Orders API: guarda os pedidos de Pix
// recebidos e devolve as orders que o teste definir. Nada sai para a rede.
type fakeMercadoPago struct {
	*httptest.Server
	mu         sync.Mutex
	created    []map[string]any
	orders     map[string]mercadopago.Order
	failCreate bool
}

func newFakeMercadoPago(t *testing.T) *fakeMercadoPago {
	t.Helper()
	f := &fakeMercadoPago{orders: map[string]mercadopago.Order{}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/orders":
			if f.failCreate {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"falha simulada"}`))
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			ref, _ := body["external_reference"].(string)
			if valid, _ := regexp.MatchString(`^[A-Za-z0-9_-]{1,64}$`, ref); !valid {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"property_value"}`))
				return
			}
			f.created = append(f.created, body)
			id := fmt.Sprintf("ORDTEST%d", len(f.created))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": id, "external_reference": body["external_reference"],
				"status": "action_required", "status_detail": "waiting_transfer",
				"transactions": map[string]any{"payments": []any{map[string]any{
					"id": "PAY" + id, "status": "action_required", "status_detail": "waiting_transfer",
					"payment_method": map[string]any{
						"id": "pix", "type": "bank_transfer", "qr_code": "pix-copia-e-cola-" + id,
						"qr_code_base64": "qr-" + id, "ticket_url": "https://example.invalid/" + id,
					},
				}}},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/orders/"):
			order, ok := f.orders[strings.TrimPrefix(r.URL.Path, "/v1/orders/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(order)
		default:
			t.Errorf("chamada inesperada ao Mercado Pago: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// setOrder define o que o Mercado Pago devolve ao consultar a order.
func (f *fakeMercadoPago) setOrder(id, externalReference string, approved bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var o mercadopago.Order
	o.ID, o.ExternalReference = id, externalReference
	o.Status, o.StatusDetail = "action_required", "waiting_transfer"
	if approved {
		o.Status, o.StatusDetail = "processed", "accredited"
	}
	p := mercadopago.Payment{ID: "PAY" + id, Status: o.Status, StatusDetail: o.StatusDetail}
	p.PaymentMethod.ID = "pix"
	o.Transactions.Payments = []mercadopago.Payment{p}
	f.orders[id] = o
}

func (f *fakeMercadoPago) createdBodies() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.created...)
}

// guestPublicServer sobe a API com banco isolado e o Mercado Pago falso.
func guestPublicServer(t *testing.T) (*Server, *pgxpool.Pool, *fakeMercadoPago) {
	t.Helper()
	s, raw := apiTestServer(t)
	mp := newFakeMercadoPago(t)
	s.mercadoPago = mercadopago.NewWithOrdersURL("token-de-teste", mp.URL+"/v1/orders", mp.Client())
	s.cfg.MercadoPagoWebhookSecret = testWebhookSecret
	return s, raw, mp
}

// openSaleMatch cria uma partida aberta, com prazo no futuro e venda de
// `slots` vagas a R$ 25,00 (R$ 25,25 com a taxa).
func openSaleMatch(t *testing.T, s *Server, raw *pgxpool.Pool, slots int) store.Match {
	t.Helper()
	ctx := context.Background()
	admin := insertUser(t, raw, "member", "admin", fmt.Sprintf("admin-%d@example.invalid", nextIP.Add(1)), "senha-de-teste-1")
	m, err := s.store.CreateMatch(ctx, store.MatchInput{
		MatchDate: "2099-01-10", StartTime: "20:00", EndTime: "22:00",
		Venue: "Arena Teste", Address: "Rua Teste, 1",
		ConfirmationDeadline: time.Now().Add(2 * time.Hour),
	}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetGuestSale(ctx, m.ID, true, slots, 2500, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	return m
}

var nextIP atomic.Int64

// publicRequest faz uma chamada sem sessão. Cada chamada vem de um IP
// diferente, salvo quando o teste fixa um (limite por IP).
func publicRequest(t *testing.T, s *Server, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	n := nextIP.Add(1)
	req.Header.Set("CF-Connecting-IP", fmt.Sprintf("10.%d.%d.%d", n>>16&255, n>>8&255, n&255))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

type createdBooking struct {
	BookingID       string    `json:"booking_id"`
	AccessToken     string    `json:"access_token"`
	Status          string    `json:"status"`
	TotalCents      int64     `json:"total_cents"`
	ExpiresAt       time.Time `json:"expires_at"`
	PixPayload      string    `json:"pix_payload"`
	PixQRCodeBase64 string    `json:"pix_qr_code_base64"`
	PixTicketURL    string    `json:"pix_ticket_url"`
}

func bookingBody(matchID, phone string) string {
	return fmt.Sprintf(`{"match_id":%q,"name":"Zé Avulso","phone":%q,"email":"Ze@Example.com"}`, matchID, phone)
}

func mustCreateBooking(t *testing.T, s *Server, matchID, phone string) createdBooking {
	t.Helper()
	rec := publicRequest(t, s, "POST", "/api/public/guest-bookings", bookingBody(matchID, phone), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("criar inscrição: %d %s", rec.Code, rec.Body)
	}
	var b createdBooking
	if err := json.NewDecoder(rec.Body).Decode(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func bookingStatus(t *testing.T, raw *pgxpool.Pool, id string) string {
	t.Helper()
	var status string
	if err := raw.QueryRow(context.Background(), `select status from guest_bookings where id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// A vitrine traz só partidas com a venda valendo, com endereço, total e
// vagas restantes, e nada sobre quem vai jogar.
func TestPublicGuestMatches(t *testing.T) {
	s, raw, _ := guestPublicServer(t)
	open := openSaleMatch(t, s, raw, 3)
	closed := openSaleMatch(t, s, raw, 3)
	if err := s.store.SetGuestSale(context.Background(), closed.ID, false, 0, 0, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	mustCreateBooking(t, s, open.ID, "11987650001")

	rec := publicRequest(t, s, "GET", "/api/public/guest-matches", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("vitrine: %d %s", rec.Code, rec.Body)
	}
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("vitrine = %v, want só a partida com venda aberta", got)
	}
	want := map[string]any{
		"match_id": open.ID, "match_date": "2099-01-10", "start_time": "20:00", "end_time": "22:00",
		"venue": "Arena Teste", "address": "Rua Teste, 1", "total_cents": float64(2525), "slots_left": float64(2),
	}
	if len(got[0]) != len(want) {
		t.Errorf("campos = %v, want exatamente %v", got[0], want)
	}
	for k, v := range want {
		if got[0][k] != v {
			t.Errorf("%s = %v, want %v", k, got[0][k], v)
		}
	}

	// Sem venda nenhuma, lista vazia (não null).
	if err := s.store.SetGuestSale(context.Background(), open.ID, false, 0, 0, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	rec = publicRequest(t, s, "GET", "/api/public/guest-matches", "", nil)
	if body := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || body != "[]" {
		t.Errorf("vitrine vazia: %d %s", rec.Code, body)
	}
}

// Inscrição feliz: reserva, Pix com a referência da inscrição, e-mail do
// formulário, total com a taxa e expiração igual à da reserva.
func TestCreateGuestBookingHandler(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 2)
	before := time.Now()

	b := mustCreateBooking(t, s, m.ID, "(11) 98765-0001")
	if b.BookingID == "" || b.Status != guests.StatusPendingPayment || b.TotalCents != 2525 {
		t.Fatalf("inscrição = %+v", b)
	}
	if d := b.ExpiresAt.Sub(before); d < 29*time.Minute || d > 31*time.Minute {
		t.Errorf("reserva vence em %v, want 30 min", d)
	}
	if b.PixPayload != "pix-copia-e-cola-ORDTEST1" || b.PixQRCodeBase64 != "qr-ORDTEST1" || b.PixTicketURL != "https://example.invalid/ORDTEST1" {
		t.Errorf("pix = %+v", b)
	}

	// Token forte devolvido uma vez; no banco, só o SHA-256.
	if len(b.AccessToken) < 43 || strings.ContainsAny(b.AccessToken, "+/=") {
		t.Errorf("access_token = %q, want base64url de 32 bytes", b.AccessToken)
	}
	var hash, orderID string
	if err := raw.QueryRow(context.Background(), `select access_token_hash, provider_order_id from guest_bookings where id = $1`,
		b.BookingID).Scan(&hash, &orderID); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(b.AccessToken))
	if hash != hex.EncodeToString(sum[:]) {
		t.Errorf("hash gravado = %q, want sha256 do token", hash)
	}
	if orderID != "ORDTEST1" {
		t.Errorf("order gravada = %q", orderID)
	}

	bodies := mp.createdBodies()
	if len(bodies) != 1 {
		t.Fatalf("pedidos de Pix = %d", len(bodies))
	}
	order := bodies[0]
	payer := order["payer"].(map[string]any)
	payment := order["transactions"].(map[string]any)["payments"].([]any)[0].(map[string]any)
	if order["external_reference"] != "guest_"+b.BookingID || order["total_amount"] != "25.25" ||
		payer["email"] != "ze@example.com" || payer["first_name"] != "Zé Avulso" ||
		payment["expiration_time"] != "PT0H30M" {
		t.Errorf("order enviada = %v", order)
	}
}

// No sandbox, o pagador segue o padrão das cobranças.
func TestCreateGuestBookingSandboxPayer(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	s.cfg.MercadoPagoTestMode = true
	m := openSaleMatch(t, s, raw, 2)
	mustCreateBooking(t, s, m.ID, "11987650001")
	payer := mp.createdBodies()[0]["payer"].(map[string]any)
	if payer["email"] != mercadoPagoSandboxPayerEmail || payer["first_name"] != "APRO" {
		t.Errorf("pagador no sandbox = %v", payer)
	}
}

func TestCreateGuestBookingErrors(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 1)
	mustCreateBooking(t, s, m.ID, "11987650001")

	post := func(body string) *httptest.ResponseRecorder {
		return publicRequest(t, s, "POST", "/api/public/guest-bookings", body, nil)
	}
	closed := openSaleMatch(t, s, raw, 5)
	if err := s.store.SetGuestSale(context.Background(), closed.ID, false, 0, 0, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		body string
		code int
		msg  string
	}{
		{"mesmo WhatsApp", bookingBody(m.ID, "+55 11 98765-0001"), 409, guests.ErrPhoneAlreadyBooked.Error()},
		{"esgotado", bookingBody(m.ID, "11987650002"), 409, guests.ErrSoldOut.Error()},
		{"venda fechada", bookingBody(closed.ID, "11987650003"), 409, guests.ErrSaleClosed.Error()},
		{"telefone inválido", bookingBody(closed.ID, "1133334444"), 400, guests.ErrInvalidPhone.Error()},
		{"nome vazio", fmt.Sprintf(`{"match_id":%q,"name":" ","phone":"11987650004","email":"a@b.co"}`, closed.ID), 400, guests.ErrInvalidName.Error()},
		{"e-mail inválido", fmt.Sprintf(`{"match_id":%q,"name":"Zé","phone":"11987650004","email":"a@b"}`, closed.ID), 400, guests.ErrInvalidEmail.Error()},
		{"JSON inválido", `{`, 400, "corpo da requisição inválido"},
		{"partida inexistente", bookingBody("00000000-0000-0000-0000-000000000000", "11987650005"), 404, ""},
		{"id malformado", bookingBody("nao-e-uuid", "11987650005"), 404, ""},
	}
	for _, tc := range cases {
		rec := post(tc.body)
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.msg) {
			t.Errorf("%s: %d %s, want %d %q", tc.name, rec.Code, rec.Body, tc.code, tc.msg)
		}
	}
	if n := len(mp.createdBodies()); n != 1 {
		t.Errorf("Pix gerados = %d, want só o da inscrição aceita", n)
	}
}

// Sem Mercado Pago configurado não há como pagar: 503 e nenhuma vaga presa.
func TestCreateGuestBookingWithoutMercadoPago(t *testing.T) {
	s, raw, _ := guestPublicServer(t)
	s.mercadoPago = mercadopago.New("")
	m := openSaleMatch(t, s, raw, 1)

	rec := publicRequest(t, s, "POST", "/api/public/guest-bookings", bookingBody(m.ID, "11987650001"), nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("sem Mercado Pago: %d %s", rec.Code, rec.Body)
	}
	var n int
	if err := raw.QueryRow(context.Background(), `select count(*) from guest_bookings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("inscrições criadas = %d, want 0", n)
	}
}

// Se o Mercado Pago falha, a reserva é expirada na hora: a vaga e o
// telefone ficam livres para uma nova tentativa.
func TestCreateGuestBookingPixFailureReleasesSlot(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 1)

	mp.mu.Lock()
	mp.failCreate = true
	mp.mu.Unlock()
	rec := publicRequest(t, s, "POST", "/api/public/guest-bookings", bookingBody(m.ID, "11987650001"), nil)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("falha do Mercado Pago: %d %s", rec.Code, rec.Body)
	}
	var id, status string
	if err := raw.QueryRow(context.Background(), `select id, status from guest_bookings`).Scan(&id, &status); err != nil {
		t.Fatal(err)
	}
	if status != guests.StatusExpired {
		t.Errorf("reserva após falha = %s, want expired", status)
	}

	mp.mu.Lock()
	mp.failCreate = false
	mp.mu.Unlock()
	b := mustCreateBooking(t, s, m.ID, "11987650001") // única vaga, mesmo telefone
	if b.Status != guests.StatusPendingPayment {
		t.Errorf("nova tentativa = %+v", b)
	}
}

func TestGetGuestBooking(t *testing.T) {
	s, raw, _ := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 2)
	b := mustCreateBooking(t, s, m.ID, "11987650001")
	path := "/api/public/guest-bookings/" + b.BookingID

	get := func(path, token string) (int, map[string]any) {
		t.Helper()
		header := map[string]string{}
		if token != "" {
			header["X-Booking-Token"] = token
		}
		rec := publicRequest(t, s, "GET", path, "", header)
		var body map[string]any
		_ = json.NewDecoder(rec.Body).Decode(&body)
		return rec.Code, body
	}

	code, body := get(path, b.AccessToken)
	if code != http.StatusOK {
		t.Fatalf("token certo: %d %v", code, body)
	}
	if body["status"] != "pending_payment" || body["total_cents"] != float64(2525) || body["expires_at"] == nil ||
		body["pix_payload"] != b.PixPayload || body["pix_qr_code_base64"] != b.PixQRCodeBase64 || body["pix_ticket_url"] != b.PixTicketURL {
		t.Errorf("inscrição = %v", body)
	}
	match, _ := body["match"].(map[string]any)
	wantMatch := map[string]any{"match_date": "2099-01-10", "start_time": "20:00", "end_time": "22:00",
		"venue": "Arena Teste", "address": "Rua Teste, 1"}
	for k, v := range wantMatch {
		if match[k] != v {
			t.Errorf("match.%s = %v, want %v", k, match[k], v)
		}
	}
	for _, leak := range []string{"phone", "email", "name", "user_id", "booking_id", "access_token"} {
		if _, ok := body[leak]; ok {
			t.Errorf("resposta expõe %s", leak)
		}
	}

	// Sem token, token errado, id inexistente ou malformado: 404, sem dizer se existe.
	for name, c := range map[string][2]string{
		"sem token":     {path, ""},
		"token errado":  {path, b.AccessToken + "x"},
		"outro id":      {"/api/public/guest-bookings/00000000-0000-0000-0000-000000000000", b.AccessToken},
		"id malformado": {"/api/public/guest-bookings/nao-e-uuid", b.AccessToken},
	} {
		if code, body := get(c[0], c[1]); code != http.StatusNotFound || body["error"] != "inscrição não encontrada" {
			t.Errorf("%s: %d %v, want 404", name, code, body)
		}
	}

	// Reserva vencida aparece expirada e sem Pix, mesmo antes do cron.
	if _, err := raw.Exec(context.Background(), `update guest_bookings set expires_at = now() - interval '1 second' where id = $1`, b.BookingID); err != nil {
		t.Fatal(err)
	}
	code, body = get(path, b.AccessToken)
	if code != http.StatusOK || body["status"] != "expired" || body["pix_payload"] != "" || body["pix_qr_code_base64"] != "" || body["pix_ticket_url"] != "" {
		t.Errorf("vencida: %d %v", code, body)
	}
}

// signedWebhook monta a notificação assinada como o Mercado Pago faz.
func signedWebhook(t *testing.T, s *Server, orderID string) *httptest.ResponseRecorder {
	t.Helper()
	const requestID, ts = "req-teste", "1704908010"
	manifest := "id:" + strings.ToLower(orderID) + ";request-id:" + requestID + ";ts:" + ts + ";"
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write([]byte(manifest))
	req := httptest.NewRequest("POST", "/api/webhooks/mercadopago?data.id="+orderID, strings.NewReader(`{}`))
	req.Header.Set("x-signature", "ts="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("x-request-id", requestID)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func countRows(t *testing.T, raw *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := raw.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Pagamento aprovado de inscrição: confirma, grava going, lança no caixa,
// agenda o WhatsApp e registra a atividade, uma vez só.
func TestWebhookConfirmsGuestBooking(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 2)
	b := mustCreateBooking(t, s, m.ID, "11987650001")

	// Sem assinatura válida, nada acontece.
	req := httptest.NewRequest("POST", "/api/webhooks/mercadopago?data.id=ORDTEST1", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("sem assinatura: %d", rec.Code)
	}

	// Ainda aguardando: só atualiza o estado do provedor.
	mp.setOrder("ORDTEST1", "guest_"+b.BookingID, false)
	if rec := signedWebhook(t, s, "ORDTEST1"); rec.Code != http.StatusOK {
		t.Fatalf("webhook pendente: %d %s", rec.Code, rec.Body)
	}
	if st := bookingStatus(t, raw, b.BookingID); st != guests.StatusPendingPayment {
		t.Errorf("antes do pagamento = %s", st)
	}

	mp.setOrder("ORDTEST1", "guest_"+b.BookingID, true)
	for i := range 2 {
		if rec := signedWebhook(t, s, "ORDTEST1"); rec.Code != http.StatusOK {
			t.Fatalf("webhook aprovado #%d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	if st := bookingStatus(t, raw, b.BookingID); st != guests.StatusConfirmed {
		t.Fatalf("depois do pagamento = %s", st)
	}
	var paymentID string
	if err := raw.QueryRow(context.Background(), `select provider_payment_id from guest_bookings where id = $1`, b.BookingID).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if paymentID != "PAYORDTEST1" {
		t.Errorf("payment gravado = %q", paymentID)
	}
	if n := countRows(t, raw, `select count(*) from match_confirmations c join guest_bookings g on g.user_id = c.user_id
		where g.id = $1 and c.match_id = $2 and c.response = 'going'`, b.BookingID, m.ID); n != 1 {
		t.Errorf("presença going = %d", n)
	}
	if n := countRows(t, raw, `select count(*) from cash_entries where booking_id = $1`, b.BookingID); n != 1 {
		t.Errorf("entradas no caixa = %d, want 1", n)
	}
	if n := countRows(t, raw, `select count(*) from guest_messages where booking_id = $1 and kind = 'confirmation'`, b.BookingID); n != 1 {
		t.Errorf("WhatsApps de confirmação = %d, want 1", n)
	}
	if n := countRows(t, raw, `select count(*) from activity_log where kind = 'guest_booking_confirmed'
		and message = 'Zé Avulso garantiu vaga avulsa na partida de 10/01'`); n != 1 {
		t.Errorf("atividades = %d, want 1", n)
	}

	// A consulta pública passa a mostrar a vaga confirmada, sem Pix.
	rec = publicRequest(t, s, "GET", "/api/public/guest-bookings/"+b.BookingID, "", map[string]string{"X-Booking-Token": b.AccessToken})
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "confirmed" || body["pix_payload"] != "" {
		t.Errorf("consulta após pagamento = %v", body)
	}
}

func TestWebhookAcceptsLegacyGuestReference(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 1)
	b := mustCreateBooking(t, s, m.ID, "11987650001")

	mp.setOrder("ORDTEST1", "guest:"+b.BookingID, true)
	if rec := signedWebhook(t, s, "ORDTEST1"); rec.Code != http.StatusOK {
		t.Fatalf("webhook legado: %d %s", rec.Code, rec.Body)
	}
	if st := bookingStatus(t, raw, b.BookingID); st != guests.StatusConfirmed {
		t.Errorf("inscrição = %s, want confirmed", st)
	}
}

// Order de inscrição desconhecida ou que não bate com a inscrição é ignorada
// sem erro (o Mercado Pago não deve reenviar para sempre).
func TestWebhookIgnoresUnknownGuestOrders(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 2)
	b := mustCreateBooking(t, s, m.ID, "11987650001")

	mp.setOrder("ORDOUTRA", "guest_"+b.BookingID, true) // order que não é a da inscrição
	mp.setOrder("ORDSUMIU", "guest_00000000-0000-0000-0000-000000000000", true)
	mp.setOrder("ORDLIXO", "guest_nao-e-uuid", true)
	for _, id := range []string{"ORDOUTRA", "ORDSUMIU", "ORDLIXO"} {
		if rec := signedWebhook(t, s, id); rec.Code != http.StatusOK {
			t.Errorf("%s: %d %s", id, rec.Code, rec.Body)
		}
	}
	if st := bookingStatus(t, raw, b.BookingID); st != guests.StatusPendingPayment {
		t.Errorf("inscrição mudou para %s", st)
	}
}

// O fluxo de cobrança (referência sem prefixo) continua igual.
func TestWebhookChargeFlowUnchanged(t *testing.T) {
	s, raw, mp := guestPublicServer(t)
	ctx := context.Background()
	admin := insertUser(t, raw, "member", "admin", "admin-cobranca@example.invalid", "senha-de-teste-1")
	if _, err := raw.Exec(ctx, `update users set name = 'Membro Pagante' where id = $1`, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.GenerateBatch(ctx, "2099-01", 5000, time.Now().AddDate(0, 0, 7), admin, billing.DefaultPixRate); err != nil {
		t.Fatal(err)
	}
	var chargeID string
	if err := raw.QueryRow(ctx, `select id from charges where user_id = $1`, admin).Scan(&chargeID); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SaveMercadoPagoOrder(ctx, chargeID, "ORDCOBRANCA", "PAYORDCOBRANCA", "action_required",
		"waiting_transfer", "pix", "https://example.invalid", "qr"); err != nil {
		t.Fatal(err)
	}

	mp.setOrder("ORDCOBRANCA", chargeID, true)
	for range 2 {
		if rec := signedWebhook(t, s, "ORDCOBRANCA"); rec.Code != http.StatusOK {
			t.Fatalf("webhook da cobrança: %d %s", rec.Code, rec.Body)
		}
	}
	charge, err := s.store.ChargeByID(ctx, chargeID)
	if err != nil {
		t.Fatal(err)
	}
	if charge.Status != "paid" {
		t.Errorf("cobrança = %s, want paid", charge.Status)
	}
	if n := countRows(t, raw, `select count(*) from activity_log where kind = 'payment'
		and message = 'Membro Pagante pagou a mensalidade de 2099-01 via PIX'`); n != 1 {
		t.Errorf("atividades de pagamento = %d, want 1", n)
	}
	if n := countRows(t, raw, `select count(*) from cash_entries`); n != 0 {
		t.Errorf("mensalidade entrou no caixa: %d", n)
	}
}

// Cada endpoint público tem seu limite por IP; estourado, 429 em português,
// e outro IP continua passando.
func TestGuestPublicRateLimit(t *testing.T) {
	s, raw, _ := guestPublicServer(t)
	m := openSaleMatch(t, s, raw, 1)
	b := mustCreateBooking(t, s, m.ID, "11987650001")

	cases := []struct {
		name, method, path, body string
		header                   map[string]string
		limit                    int
	}{
		{"vitrine", "GET", "/api/public/guest-matches", "", nil, guestListLimit},
		{"inscrição", "POST", "/api/public/guest-bookings", `{}`, nil, guestCreateLimit},
		{"consulta", "GET", "/api/public/guest-bookings/" + b.BookingID, "",
			map[string]string{"X-Booking-Token": b.AccessToken}, guestStatusLimit},
	}
	for i, tc := range cases {
		ip := fmt.Sprintf("203.0.113.%d", i+1)
		header := map[string]string{"CF-Connecting-IP": ip}
		for k, v := range tc.header {
			header[k] = v
		}
		for n := range tc.limit {
			if rec := publicRequest(t, s, tc.method, tc.path, tc.body, header); rec.Code == http.StatusTooManyRequests {
				t.Fatalf("%s: 429 na chamada %d, limite %d", tc.name, n+1, tc.limit)
			}
		}
		rec := publicRequest(t, s, tc.method, tc.path, tc.body, header)
		if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "muitas requisições") {
			t.Errorf("%s acima do limite: %d %s", tc.name, rec.Code, rec.Body)
		}
		header["CF-Connecting-IP"] = "198.51.100.1" + fmt.Sprint(i)
		if rec := publicRequest(t, s, tc.method, tc.path, tc.body, header); rec.Code == http.StatusTooManyRequests {
			t.Errorf("%s: outro IP bloqueado", tc.name)
		}
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:41234"
	if got := clientIP(req); got != "172.18.0.5" {
		t.Errorf("sem Cloudflare: %q", got)
	}
	req.Header.Set("CF-Connecting-IP", " 2001:db8::1 ")
	if got := clientIP(req); got != "2001:db8::1" {
		t.Errorf("com Cloudflare: %q", got)
	}
}
