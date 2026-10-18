// Package jobs agenda as rotinas de manutenção do sistema.
package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"futdarapaziada/api/internal/notify"
	"futdarapaziada/api/internal/push"
	"futdarapaziada/api/internal/store"
)

type Runner struct {
	store        *store.Store
	sender       notify.Sender
	cron         *cron.Cron
	push         *push.Client
	reminderLead time.Duration
}

func New(st *store.Store, sender notify.Sender) *Runner {
	return &Runner{store: st, sender: sender, cron: cron.New()}
}

// Start registra e inicia os jobs. Erros de agenda são bugs de código, então panic.
func (r *Runner) Start() {
	mustAdd := func(spec string, job func()) {
		if _, err := r.cron.AddFunc(spec, job); err != nil {
			panic(fmt.Sprintf("agendando job %q: %v", spec, err))
		}
	}

	if r.push.Enabled() {
		mustAdd("* * * * *", r.sendMatchPush)
		mustAdd("* * * * *", r.sendBroadcastPush)
	}
	mustAdd("* * * * *", r.closeExpiredConfirmations) // a cada minuto
	mustAdd("*/10 * * * *", r.closeExpiredVoting)
	mustAdd("*/10 * * * *", r.openUnopenedVoting)     // FUT-001
	mustAdd("15 0 * * *", r.markOverdueAndInactivate) // diário, 00h15
	mustAdd("*/5 * * * *", r.sendDueNotifications)
	mustAdd("15 8 * * *", r.materializeRecurring) // diário, 05:15 SP (FUT-004)

	r.cron.Start()
}

func (r *Runner) Stop() { r.cron.Stop() }

func (r *Runner) closeExpiredConfirmations() {
	ctx := context.Background()
	ids, err := r.store.CloseExpiredConfirmations(ctx)
	if err != nil {
		log.Printf("job fechar confirmações: %v", err)
		return
	}
	for range ids {
		r.store.LogActivity(ctx, nil, "confirmations_closed", "Confirmações encerradas automaticamente (prazo atingido)")
	}
}

func (r *Runner) closeExpiredVoting() {
	ctx := context.Background()
	ids, err := r.store.CloseExpiredVoting(ctx)
	if err != nil {
		log.Printf("job fechar votação: %v", err)
		return
	}
	for _, id := range ids {
		results, err := r.store.VoteResults(ctx, id)
		if err != nil {
			continue
		}
		for _, res := range results {
			// FUT-005: label por categoria (inclui "melhor gol")
			label := "artilheiro"
			switch res.Category {
			case "worst_player":
				label = "perna de pau"
			case "best_goal":
				label = "melhor gol"
			}
			for _, winner := range res.Winners {
				r.store.LogActivity(ctx, nil, "vote_result",
					fmt.Sprintf("%s foi eleito %s da partida", winner.Name, label))
			}
		}
	}
}

// openUnopenedVoting (FUT-001): abre a votação de partidas que já acabaram e
// dependiam de clique manual do admin. O delay e a duração vêm de settings —
// voting_open_delay_min (default 0, abre no próprio tick pós-fim) e
// voting_close_days (default 2, condição já documentada na spec).
func (r *Runner) openUnopenedVoting() {
	ctx := context.Background()
	delayMin := r.store.SettingInt(ctx, "voting_open_delay_min", 0)
	days := r.store.SettingInt(ctx, "voting_close_days", 2)

	ids, err := r.store.OpenExpiredVoting(ctx, delayMin, days)
	if err != nil {
		log.Printf("job abrir votação: %v", err)
		return
	}
	for _, id := range ids {
		if m, err := r.store.MatchByID(ctx, id); err == nil {
			r.store.LogActivity(ctx, nil, "voting_opened",
				fmt.Sprintf("Votação aberta automaticamente — partida do dia %s",
					m.MatchDate[8:10]+"/"+m.MatchDate[5:7]))
		}
	}
}

// materializeRecurring (FUT-004): cria as 4 próximas ocorrências de cada
// template ativo. Roda às 05:15 horário do processo (TZ=America/Sao_Paulo
// em produção). Na primeira execução após migrate, o seed cria o template
// do grupo se ainda não existir.
func (r *Runner) materializeRecurring() {
	ctx := context.Background()
	if err := r.store.RecurringSeed(ctx); err != nil {
		log.Printf("seed recorrente: %v", err)
	}
	n, err := r.store.MaterializeRecurringMatches(ctx, 4)
	if err != nil {
		log.Printf("job materializar recorrentes: %v", err)
		return
	}
	if n > 0 {
		log.Printf("materializadas %d partidas recorrentes", n)
	}
}

// markOverdueAndInactivate marca cobranças vencidas, agenda o lembrete de
// WhatsApp para cada uma e inativa quem estourou o prazo de tolerância
// (overdue_inactivate_days; 0 = inativa no dia seguinte ao vencimento).
func (r *Runner) markOverdueAndInactivate() {
	ctx := context.Background()
	overdue, err := r.store.MarkOverdueCharges(ctx)
	if err != nil {
		log.Printf("job marcar vencidas: %v", err)
		return
	}
	r.scheduleOverdueReminders(ctx, overdue)

	graceDays := r.store.SettingInt(ctx, "overdue_inactivate_days", 0)
	users, err := r.store.UsersToInactivate(ctx, graceDays)
	if err != nil {
		log.Printf("job inativar inadimplentes: %v", err)
		return
	}
	for _, u := range users {
		err := r.store.ChangeUserStatus(ctx, u.ID, "inactive", "inadimplência: mensalidade vencida", nil)
		if err != nil {
			log.Printf("inativando %s: %v", u.Name, err)
			continue
		}
		r.store.LogActivity(ctx, nil, "user_inactivated",
			fmt.Sprintf("%s foi inativado por inadimplência", u.Name))
	}
}

const defaultOverdueTemplate = "Olá, {{nome}}. A mensalidade de {{mes_referencia}}, no valor de {{valor}}, venceu em {{data_vencimento}} e ainda está em aberto. Regularize para evitar a inativação."

// scheduleOverdueReminders agenda o lembrete de vencido de cada cobrança que
// acabou de transicionar — a fila (sendDueNotifications) faz o envio.
func (r *Runner) scheduleOverdueReminders(ctx context.Context, overdue []store.OverdueCharge) {
	if len(overdue) == 0 {
		return
	}
	template := r.store.SettingString(ctx, "overdue_template", defaultOverdueTemplate)
	for _, o := range overdue {
		phone, err := notify.NormalizeNumber(o.UserPhone)
		if err != nil {
			log.Printf("lembrete de vencido: %s sem WhatsApp válido (%q): %v", o.UserName, o.UserPhone, err)
			continue
		}
		dueDate, err := time.Parse("2006-01-02", o.DueDate)
		if err != nil {
			log.Printf("lembrete de vencido: vencimento inválido na cobrança %s: %v", o.ID, err)
			continue
		}
		message := renderTemplate(template, map[string]string{
			"{{nome}}":            o.UserName,
			"{{mes_referencia}}":  o.ReferenceMonth,
			"{{valor}}":           formatCentsBR(o.AmountCents),
			"{{data_vencimento}}": dueDate.Format("02/01/2006"),
		})
		if _, err := r.store.ScheduleNotification(ctx, o.UserID, &o.ID, phone, message, time.Now()); err != nil {
			log.Printf("agendando lembrete de vencido para %s: %v", o.UserName, err)
		}
	}
}

func renderTemplate(template string, replacements map[string]string) string {
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}

// formatCentsBR formata centavos como "R$ 80,00".
func formatCentsBR(cents int64) string {
	return fmt.Sprintf("R$ %d,%02d", cents/100, cents%100)
}

func (r *Runner) sendDueNotifications() {
	ctx := context.Background()
	due, err := r.store.DueNotifications(ctx)
	if err != nil {
		log.Printf("job lembretes: %v", err)
		return
	}
	for _, n := range due {
		providerID, err := r.sender.Send(n.Phone, n.Message)
		if err != nil {
			_ = r.store.MarkNotificationFailed(ctx, n.ID, err.Error())
			continue
		}
		_ = r.store.MarkNotificationSent(ctx, n.ID, providerID)
	}
}
