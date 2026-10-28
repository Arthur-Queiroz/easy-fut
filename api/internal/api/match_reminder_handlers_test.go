package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestManualMatchReminderRoutes(t *testing.T) {
	s, raw := apiTestServer(t)
	s.cfg.AppURL = "https://fut.example"
	ctx := context.Background()
	admin := insertUser(t, raw, "member", "admin", "reminder-admin@example.invalid", "senha-de-teste-1")
	player := insertUser(t, raw, "member", "player", "reminder-player@example.invalid", "senha-de-teste-1")
	if _, err := raw.Exec(ctx, `update users set phone='11999990002' where id=$1`, player); err != nil {
		t.Fatal(err)
	}
	var matchID string
	if err := raw.QueryRow(ctx, `insert into matches(match_date,start_time,end_time,venue,confirmation_deadline,created_by)
		values(current_date,'20:00','22:00','Arena 1',$1,$2) returning id`, time.Now().Add(time.Hour), admin).Scan(&matchID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(ctx, `insert into match_confirmations(match_id,user_id) values($1,$2)`, matchID, player); err != nil {
		t.Fatal(err)
	}
	path := "/api/admin/matches/" + matchID + "/whatsapp-reminders"
	body := fmt.Sprintf(`{"kind":"confirmation","user_ids":[%q]}`, player)
	if rec := sessionRequest(t, s, http.MethodPost, path, body, player); rec.Code != http.StatusForbidden {
		t.Fatalf("jogador enviou lembrete: %d", rec.Code)
	}
	if rec := sessionRequest(t, s, http.MethodGet, path+"?kind=confirmation", "", admin); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), player) {
		t.Fatalf("prévia: %d %s", rec.Code, rec.Body.String())
	}
	if rec := sessionRequest(t, s, http.MethodPost, path, `{"kind":"other","user_ids":["`+player+`"]}`, admin); rec.Code != http.StatusBadRequest {
		t.Fatalf("tipo inválido: %d", rec.Code)
	}
	if rec := sessionRequest(t, s, http.MethodPost, path, body, admin); rec.Code != http.StatusAccepted {
		t.Fatalf("enfileirar: %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := raw.QueryRow(ctx, `select count(*) from notifications where match_id=$1 and user_id=$2 and reminder_kind='confirmation'`, matchID, player).Scan(&count); err != nil || count != 1 {
		t.Fatalf("fila: count=%d err=%v", count, err)
	}
	if rec := sessionRequest(t, s, http.MethodPost, path, body, admin); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("envio duplicado: %d", rec.Code)
	}
}
