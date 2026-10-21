package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"futdarapaziada/api/internal/guests"
	"futdarapaziada/api/internal/store"
)

// Participação avulsa — admin (fase B): venda por partida, tela Avulsos,
// baixa do reembolso e caixa do grupo. Regras em internal/guests.

// handleGuestSale abre, reajusta ou fecha a venda de vagas avulsas da
// partida. Ao abrir, congela o preço vigente (guest_price_cents) e o total
// com a mesma taxa do Pix repassada nas mensalidades (pix_fee_bps).
func (s *Server) handleGuestSale(w http.ResponseWriter, r *http.Request) {
	matchID := r.PathValue("id")
	if _, err := uuid.Parse(matchID); err != nil {
		writeError(w, http.StatusNotFound, "registro não encontrado")
		return
	}
	var body struct {
		Open  *bool `json:"open"`
		Slots *int  `json:"slots"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Open == nil || (*body.Open && body.Slots == nil) {
		writeError(w, http.StatusBadRequest, "informe open e, para abrir, slots")
		return
	}
	slots := 0
	if body.Slots != nil {
		slots = *body.Slots
	}
	if slots < 0 || slots > guests.MaxSlots {
		writeStoreError(w, guests.ErrInvalidSlots)
		return
	}

	ctx := r.Context()
	price := guests.PriceOrDefault(s.store.SettingString(ctx, "guest_price_cents", ""))
	match, err := s.store.UpdateGuestSale(ctx, matchID, *body.Open, slots, price, s.pixFeeRate(ctx), time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}

	admin := currentUser(r)
	if *body.Open {
		s.store.LogActivity(ctx, &admin.ID, "guest_sale_opened",
			fmt.Sprintf("%s abriu %d vagas avulsas na partida de %s", admin.Name, match.GuestSlots, formatDateBR(match.MatchDate)))
	} else {
		s.store.LogActivity(ctx, &admin.ID, "guest_sale_closed",
			fmt.Sprintf("%s fechou a venda de vagas avulsas da partida de %s", admin.Name, formatDateBR(match.MatchDate)))
	}
	writeJSON(w, http.StatusOK, match)
}

// handleListGuestBookings é a tela Avulsos: filtros por partida e estado e
// busca por nome ou WhatsApp.
func (s *Server) handleListGuestBookings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.GuestBookingFilter{MatchID: q.Get("match_id"), Status: q.Get("status"), Query: q.Get("q")}
	if filter.MatchID != "" {
		if _, err := uuid.Parse(filter.MatchID); err != nil {
			writeError(w, http.StatusBadRequest, "match_id inválido")
			return
		}
	}
	if filter.Status != "" && !guests.ValidStatus(filter.Status) {
		writeError(w, http.StatusBadRequest, "status de inscrição inválido")
		return
	}
	bookings, err := s.store.ListGuestBookings(r.Context(), filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, orEmpty(bookings))
}

// handleGuestRefunded registra que o admin estornou o Pix no Mercado Pago:
// a inscrição vai de refund_pending para refunded e o caixa ganha a saída.
// Repetir a chamada devolve a mesma inscrição sem lançar nada.
func (s *Server) handleGuestRefunded(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusNotFound, "registro não encontrado")
		return
	}
	admin := currentUser(r)
	changed, err := s.store.MarkGuestBookingRefunded(r.Context(), id, admin.ID, time.Now())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	booking, err := s.store.AdminGuestBookingByID(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if changed {
		s.store.LogActivity(r.Context(), &admin.ID, "guest_refunded",
			fmt.Sprintf("%s marcou o estorno da inscrição avulsa de %s", admin.Name, booking.Name))
	}
	writeJSON(w, http.StatusOK, booking)
}

// validDate confere uma data de calendário YYYY-MM-DD.
func validDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// handleCash devolve o saldo do caixa inteiro e os lançamentos do período
// (from e to opcionais, inclusivos).
func (s *Server) handleCash(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if (from != "" && !validDate(from)) || (to != "" && !validDate(to)) {
		writeError(w, http.StatusBadRequest, "informe from e to no formato YYYY-MM-DD")
		return
	}
	// Datas YYYY-MM-DD comparam como texto.
	if from != "" && to != "" && from > to {
		writeError(w, http.StatusBadRequest, "from precisa ser anterior ou igual a to")
		return
	}
	report, err := s.store.CashReport(r.Context(), from, to)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	report.Entries = orEmpty(report.Entries)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAddCashExpense(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Description string `json:"description"`
		AmountCents int64  `json:"amount_cents"`
		OccurredOn  string `json:"occurred_on"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validDate(body.OccurredOn) {
		writeError(w, http.StatusBadRequest, "informe a data da despesa no formato YYYY-MM-DD")
		return
	}
	admin := currentUser(r)
	entry, err := s.store.AddCashExpense(r.Context(), body.Description, body.AmountCents, body.OccurredOn, admin.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

// handleDeleteCashExpense remove uma despesa lançada por engano; entradas e
// estornos de inscrições respondem 404.
func (s *Server) handleDeleteCashExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusNotFound, "registro não encontrado")
		return
	}
	if err := s.store.DeleteCashExpense(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
