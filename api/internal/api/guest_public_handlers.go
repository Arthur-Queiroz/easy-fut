package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"futdarapaziada/api/internal/auth"
	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/mercadopago"
	"futdarapaziada/api/internal/store"
)

// Participação avulsa — público (fase A): vitrine das partidas com venda
// aberta, inscrição com Pix e acompanhamento da inscrição. Não há sessão: o
// acesso à inscrição é o token devolvido uma única vez na criação, guardado
// no banco só como SHA-256.

const (
	// guestOrderRefPrefix marca no external_reference do Mercado Pago as
	// orders de inscrição avulsa ("guest_<booking_id>"); sem o prefixo, a
	// order é de uma cobrança de membro.
	guestOrderRefPrefix = "guest_"
	// Aceita notificações de orders criadas antes da correção do prefixo.
	legacyGuestOrderRefPrefix = "guest:"
	bookingTokenHeader        = "X-Booking-Token"
)

// Limites por IP dos endpoints públicos (janela deslizante, em memória, como
// o login). A vitrine e a consulta aguentam o polling da PWA (a cada 5 s, 12
// por minuto) com folga para mais de uma aba. A criação cabe um grupo de
// amigos atrás do mesmo IP, com erros de digitação, mas impede que um IP
// prenda muitas vagas: cada reserva dura 30 minutos, a mesma janela.
const (
	guestListLimit    = 60
	guestListWindow   = time.Minute
	guestCreateLimit  = 10
	guestCreateWindow = guests.ReservationTTL
	guestStatusLimit  = 30
	guestStatusWindow = time.Minute
)

type guestPublicLimiters struct {
	list, create, status *auth.LoginLimiter
}

func newGuestPublicLimiters() guestPublicLimiters {
	return guestPublicLimiters{
		list:   auth.NewLoginLimiter(guestListLimit, guestListWindow),
		create: auth.NewLoginLimiter(guestCreateLimit, guestCreateWindow),
		status: auth.NewLoginLimiter(guestStatusLimit, guestStatusWindow),
	}
}

// clientIP identifica quem chama. Em produção a requisição passa pelo
// Cloudflare Tunnel e pelo Caddy, então RemoteAddr é o proxy interno; o
// Cloudflare sempre sobrescreve CF-Connecting-IP com o IP real, e a API não
// é exposta fora dele. Sem o header (local), vale RemoteAddr.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limitByIP recusa com 429 quem passou do limite do endpoint.
func limitByIP(l *auth.LoginLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(clientIP(r)) {
				writeError(w, http.StatusTooManyRequests, "muitas requisições; aguarde alguns minutos e tente de novo")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type publicGuestMatch struct {
	MatchID    string `json:"match_id"`
	MatchDate  string `json:"match_date"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Venue      string `json:"venue"`
	Address    string `json:"address"`
	TotalCents int64  `json:"total_cents"`
	SlotsLeft  int    `json:"slots_left"`
}

// handlePublicGuestMatches lista as partidas com venda aberta. Decisão do
// grupo: mostra tudo da partida (inclusive endereço), nunca quem vai jogar.
func (s *Server) handlePublicGuestMatches(w http.ResponseWriter, r *http.Request) {
	sales, err := s.store.OpenGuestSales(r.Context(), time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := make([]publicGuestMatch, 0, len(sales))
	for _, m := range sales {
		out = append(out, publicGuestMatch{
			MatchID: m.MatchID, MatchDate: m.MatchDate, StartTime: m.StartTime, EndTime: m.EndTime,
			Venue: m.Venue, Address: m.Address, TotalCents: m.TotalCents,
			SlotsLeft: guests.SlotsLeft(m.Slots, m.ActiveBookings),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createdGuestBooking struct {
	BookingID       string    `json:"booking_id"`
	AccessToken     string    `json:"access_token"`
	Status          string    `json:"status"`
	TotalCents      int64     `json:"total_cents"`
	ExpiresAt       time.Time `json:"expires_at"`
	PixPayload      string    `json:"pix_payload"`
	PixQRCodeBase64 string    `json:"pix_qr_code_base64"`
	PixTicketURL    string    `json:"pix_ticket_url"`
}

// handleCreateGuestBooking reserva a vaga (30 min), gera o Pix com a mesma
// expiração e devolve o token de acompanhamento, que não é guardado.
//
// Se o Pix não puder ser gerado, a reserva é expirada na hora (vaga e
// telefone livres para nova tentativa) e a resposta é 502. O avulso nunca
// viu um QR dessa reserva, então não há pagamento possível a perder.
func (s *Server) handleCreateGuestBooking(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MatchID string `json:"match_id"`
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		Email   string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	// Antes de reservar: sem Mercado Pago, a vaga ficaria presa sem Pix.
	if !s.mercadoPago.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "a integração com Mercado Pago ainda não está configurada")
		return
	}
	token, err := newBookingAccessToken()
	if err != nil {
		log.Printf("gerando token da inscrição avulsa: %v", err)
		writeError(w, http.StatusInternalServerError, "não foi possível preparar a inscrição")
		return
	}

	ctx := r.Context()
	now := time.Now()
	booking, err := s.store.CreateGuestBooking(ctx, store.GuestBookingInput{
		MatchID: body.MatchID, Name: body.Name, Phone: body.Phone, Email: body.Email,
		AccessTokenHash: hashBookingAccessToken(token),
	}, now)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	release := func() {
		// Sem o contexto da requisição: o cliente pode ter desistido, e a
		// vaga precisa ser liberada mesmo assim.
		if err := s.store.ReleaseGuestBooking(context.WithoutCancel(ctx), booking.ID, time.Now()); err != nil {
			log.Printf("liberando reserva avulsa %s: %v", booking.ID, err)
		}
	}
	order, payment, err := s.createGuestPix(ctx, booking, now)
	if err != nil {
		log.Printf("criando Pix da inscrição avulsa %s: %v", booking.ID, err)
		release()
		var providerError *mercadopago.APIError
		if s.cfg.MercadoPagoTestMode && errors.As(err, &providerError) {
			message := providerError.Message
			if message == "" {
				message = fmt.Sprintf("HTTP %d", providerError.StatusCode)
			}
			writeError(w, http.StatusBadGateway, "Mercado Pago sandbox recusou o Pix: "+message)
			return
		}
		writeError(w, http.StatusBadGateway, "não foi possível gerar o Pix agora; tente novamente")
		return
	}
	pending, err := s.store.MarkGuestBookingPendingPayment(ctx, booking.ID, store.GuestPix{
		OrderID: order.ID, PaymentID: payment.ID, Status: order.Status, StatusDetail: order.StatusDetail,
		Payload: payment.PaymentMethod.QRCode, TicketURL: payment.PaymentMethod.TicketURL,
		QRCodeBase64: payment.PaymentMethod.QRCodeBase64,
	})
	if err != nil {
		log.Printf("gravando Pix da inscrição avulsa %s: %v", booking.ID, err)
		release()
		writeError(w, http.StatusInternalServerError, "não foi possível registrar o Pix; tente novamente")
		return
	}

	writeJSON(w, http.StatusCreated, createdGuestBooking{
		BookingID: pending.ID, AccessToken: token, Status: pending.Status,
		TotalCents: pending.TotalCents, ExpiresAt: pending.ExpiresAt,
		PixPayload: pending.PixPayload, PixQRCodeBase64: pending.PixQRCodeBase64,
		PixTicketURL: pending.PixTicketURL,
	})
}

// createGuestPix pede o Pix ao Mercado Pago para a inscrição: total com a
// taxa embutida, pagador com o e-mail do formulário (no sandbox, o mesmo
// pagador de teste das cobranças) e expiração no fim da reserva.
func (s *Server) createGuestPix(ctx context.Context, b store.GuestBooking, now time.Time) (mercadopago.Order, mercadopago.Payment, error) {
	payerEmail, payerFirstName := b.Email, b.Name
	if s.cfg.MercadoPagoTestMode {
		payerEmail, payerFirstName = mercadoPagoSandboxPayerEmail, "APRO"
	}
	idempotencyKey, err := mercadoPagoIdempotencyKey()
	if err != nil {
		return mercadopago.Order{}, mercadopago.Payment{}, fmt.Errorf("gerando chave de idempotência: %w", err)
	}
	order, err := s.mercadoPago.CreatePixOrder(ctx, mercadopago.CreatePixOrderInput{
		Amount: amountInReais(b.TotalCents),
		// O campo do cliente se chama ChargeID, mas é o external_reference;
		// o prefixo separa a inscrição das cobranças no webhook.
		ChargeID:           guestOrderRefPrefix + b.ID,
		PayerEmail:         payerEmail,
		PayerFirstName:     payerFirstName,
		ExpirationDuration: guestPixExpiration(b.ExpiresAt.Sub(now)),
		IdempotencyKey:     idempotencyKey,
	})
	if err != nil {
		return mercadopago.Order{}, mercadopago.Payment{}, err
	}
	payment, ok := order.PixPayment()
	if !ok || payment.PaymentMethod.QRCode == "" || order.ID == "" {
		return mercadopago.Order{}, mercadopago.Payment{}, errors.New("o Mercado Pago retornou uma order Pix incompleta")
	}
	return order, payment, nil
}

// guestPixExpiration escreve a duração da reserva no formato ISO 8601 que o
// Mercado Pago aceita (o mesmo de pixExpiration). A reserva é criada com o
// mesmo `now`, então o resultado é exatamente guests.ReservationTTL.
func guestPixExpiration(d time.Duration) string {
	minutes := int(d.Round(time.Minute).Minutes())
	return fmt.Sprintf("PT%dH%dM", minutes/60, minutes%60)
}

type publicBookingMatch struct {
	MatchDate string `json:"match_date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Venue     string `json:"venue"`
	Address   string `json:"address"`
}

type publicGuestBooking struct {
	Status          string             `json:"status"`
	TotalCents      int64              `json:"total_cents"`
	ExpiresAt       time.Time          `json:"expires_at"`
	Match           publicBookingMatch `json:"match"`
	PixPayload      string             `json:"pix_payload"`
	PixQRCodeBase64 string             `json:"pix_qr_code_base64"`
	PixTicketURL    string             `json:"pix_ticket_url"`
}

// handleGetGuestBooking mostra a inscrição a quem tem o token. Token ausente
// ou errado responde igual a inscrição inexistente (404), para não revelar
// quais ids existem. Os campos do Pix vêm vazios fora de pending_payment.
func (s *Server) handleGetGuestBooking(w http.ResponseWriter, r *http.Request) {
	notFound := func() { writeError(w, http.StatusNotFound, "inscrição não encontrada") }
	token := r.Header.Get(bookingTokenHeader)
	if token == "" {
		notFound()
		return
	}
	booking, err := s.store.GuestBookingByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) || isMalformedID(err) {
		notFound()
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if !bookingTokenMatches(token, booking.AccessTokenHash) {
		notFound()
		return
	}
	match, err := s.store.MatchByID(r.Context(), booking.MatchID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	status := guests.PublicStatus(guests.Booking{Status: booking.Status, ExpiresAt: booking.ExpiresAt}, time.Now())
	out := publicGuestBooking{
		Status: status, TotalCents: booking.TotalCents, ExpiresAt: booking.ExpiresAt,
		Match: publicBookingMatch{
			MatchDate: match.MatchDate, StartTime: match.StartTime, EndTime: match.EndTime,
			Venue: match.Venue, Address: match.Address,
		},
	}
	if guests.ShowsPix(status) {
		out.PixPayload = booking.PixPayload
		out.PixQRCodeBase64 = booking.PixQRCodeBase64
		out.PixTicketURL = booking.PixTicketURL
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGuestPaymentNotification é o ramo do webhook do Mercado Pago para
// orders de inscrição avulsa. A assinatura já foi validada e a order já foi
// consultada no Mercado Pago por handleMercadoPagoWebhook. Order que não
// corresponde a uma inscrição é ignorada com 200 (reenviar não ajudaria);
// erro do banco devolve 5xx para o Mercado Pago tentar de novo.
func (s *Server) handleGuestPaymentNotification(w http.ResponseWriter, r *http.Request, order mercadopago.Order, bookingID string) {
	ctx := r.Context()
	booking, err := s.store.GuestBookingByID(ctx, bookingID)
	if errors.Is(err, store.ErrNotFound) || isMalformedID(err) {
		log.Printf("webhook Mercado Pago ignorado: inscrição avulsa %q não encontrada", bookingID)
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	payment, ok := order.PixPayment()
	if !ok || booking.ProviderOrderID == "" || booking.ProviderOrderID != order.ID {
		log.Printf("webhook Mercado Pago ignorado: order %s não corresponde à inscrição avulsa %s", order.ID, booking.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := s.store.UpdateGuestProviderStatus(ctx, booking.ID, order.ID, payment.ID, order.Status, order.StatusDetail); err != nil {
		writeStoreError(w, err)
		return
	}
	if !order.IsApproved() {
		w.WriteHeader(http.StatusOK)
		return
	}

	updated, changed, err := s.store.ConfirmGuestPayment(ctx, booking.ID, order.ID, payment.ID, time.Now())
	if errors.Is(err, store.ErrNotFound) {
		log.Printf("webhook Mercado Pago ignorado: order %s não corresponde à inscrição avulsa %s", order.ID, booking.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if changed {
		switch updated.Status {
		case guests.StatusConfirmed:
			s.logGuestConfirmed(ctx, updated)
		case guests.StatusRefundPending:
			log.Printf("inscrição avulsa %s paga sem vaga (%s): reembolso pendente", updated.ID, updated.RefundReason)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) logGuestConfirmed(ctx context.Context, b store.GuestBooking) {
	match, err := s.store.MatchByID(ctx, b.MatchID)
	if err != nil {
		log.Printf("atividade da inscrição avulsa %s: %v", b.ID, err)
		return
	}
	s.store.LogActivity(ctx, nil, "guest_booking_confirmed",
		fmt.Sprintf("%s garantiu vaga avulsa na partida de %s/%s", b.Name, match.MatchDate[8:10], match.MatchDate[5:7]))
}

// newBookingAccessToken gera o token de acompanhamento: 32 bytes aleatórios
// em base64url (43 caracteres).
func newBookingAccessToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashBookingAccessToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// bookingTokenMatches compara em tempo constante o hash do token recebido com
// o gravado na inscrição.
func bookingTokenMatches(token, storedHash string) bool {
	if storedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashBookingAccessToken(token)), []byte(storedHash)) == 1
}
