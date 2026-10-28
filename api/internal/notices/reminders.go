package notices

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidReminderKind = errors.New("tipo de lembrete inválido")
	ErrReminderClosed      = errors.New("o prazo para este lembrete já encerrou")
)

const (
	Confirmation = "confirmation"
	Vote         = "vote"
)

func CheckMatchReminder(kind, status string, deadline, votingClosesAt *time.Time, now time.Time) error {
	switch kind {
	case Confirmation:
		if status != "open" || deadline == nil || !now.Before(*deadline) {
			return ErrReminderClosed
		}
	case Vote:
		if status != "voting" || votingClosesAt == nil || !now.Before(*votingClosesAt) {
			return ErrReminderClosed
		}
	default:
		return ErrInvalidReminderKind
	}
	return nil
}

func MatchReminderMessage(kind, name, matchDate, venue, appURL, matchID string) string {
	date := matchDate
	if parsed, err := time.Parse("2006-01-02", matchDate); err == nil {
		date = parsed.Format("02/01")
	}
	link := strings.TrimRight(appURL, "/") + "/partida?match=" + matchID
	if kind == Confirmation {
		return fmt.Sprintf("Oi, %s! Você ainda não confirmou presença na partida de %s, em %s. Confirme pelo app antes do prazo: %s", name, date, venue, link)
	}
	return fmt.Sprintf("Oi, %s! A votação da partida de %s, em %s, está aberta e seu voto ainda não foi registrado. Vote pelo app antes do prazo: %s", name, date, venue, link)
}
