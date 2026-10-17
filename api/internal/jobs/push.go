package jobs

import (
	"context"
	"errors"
	"fmt"
	"futdarapaziada/api/internal/push"
	"futdarapaziada/api/internal/store"
	"log"
	"time"
)

func (r *Runner) ConfigurePush(client *push.Client, reminderHours int) {
	r.push = client
	r.reminderLead = time.Duration(reminderHours) * time.Hour
}
func matchPushMessage(d store.PushDelivery) push.Message {
	date := d.MatchDate
	if parsed, err := time.Parse("2006-01-02", date); err == nil {
		date = parsed.Format("02/01")
	}
	title := "Nova partida! Confirme sua presença"
	if d.Kind == "reminder" {
		title = "Você ainda não confirmou sua presença"
	}
	return push.Message{Title: title, Body: fmt.Sprintf("%s às %s · %s. Marque se você vai participar.", date, d.StartTime, d.Venue), URL: "/partida?match=" + d.MatchID, Tag: "match-" + d.MatchID + "-" + d.Kind}
}
func (r *Runner) sendMatchPush() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	if err := r.store.QueueMatchReminders(ctx, r.reminderLead); err != nil {
		log.Printf("push: agendando lembretes: %v", err)
		return
	}
	for i := 0; i < 50 && ctx.Err() == nil; i++ {
		d, err := r.store.ClaimPushDelivery(ctx, r.reminderLead)
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		if err != nil {
			log.Printf("push: obtendo envio: %v", err)
			return
		}
		ttl := int(time.Until(d.Deadline).Seconds())
		if ttl < 1 {
			continue
		}
		if ttl > 86400 {
			ttl = 86400
		}
		status, sendErr := r.push.Send(ctx, d.Subscription, matchPushMessage(d), ttl)
		// Persist even if the worker's deadline expired while contacting the service.
		completeCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		err = r.store.CompletePushDelivery(completeCtx, d, status, sendErr)
		done()
		if err != nil {
			log.Printf("push: registrando envio: %v", err)
		}
		if sendErr != nil {
			log.Printf("push: envio %s falhou: %v", d.ID, sendErr)
		}
	}
}
func broadcastPushMessage(d store.BroadcastDelivery) push.Message {
	return push.Message{Title: d.Title, Body: d.Body, URL: "/", Tag: "broadcast-" + d.BroadcastID}
}
func (r *Runner) sendBroadcastPush() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	for i := 0; i < 50 && ctx.Err() == nil; i++ {
		d, err := r.store.ClaimBroadcastDelivery(ctx)
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		if err != nil {
			log.Printf("push: obtendo aviso: %v", err)
			return
		}
		ttl := int(time.Until(d.CreatedAt.Add(store.BroadcastMaxAge)).Seconds())
		if ttl < 1 {
			continue
		}
		status, sendErr := r.push.Send(ctx, d.Subscription, broadcastPushMessage(d), ttl)
		completeCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		err = r.store.CompleteBroadcastDelivery(completeCtx, d, status, sendErr)
		done()
		if err != nil {
			log.Printf("push: registrando aviso: %v", err)
		}
		if sendErr != nil {
			log.Printf("push: aviso %s falhou: %v", d.ID, sendErr)
		}
	}
}
