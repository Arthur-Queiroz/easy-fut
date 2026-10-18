package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicateVote indica voto repetido no mesmo jogador dentro da categoria.
var ErrDuplicateVote = errors.New("você já votou neste jogador nesta categoria")

// CastVote grava o voto do usuário na categoria. Cada participante pode votar
// em até 2 jogadores distintos por categoria — o limite é validado no handler
// (handleVote) e a duplicidade é garantida pela constraint unique do banco.
func (s *Store) CastVote(ctx context.Context, matchID, voterID, category, candidateID string) error {
	_, err := s.pool.Exec(ctx, `
		insert into votes (match_id, voter_id, category, candidate_id)
		values ($1, $2, $3, $4)`,
		matchID, voterID, category, candidateID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrDuplicateVote
	}
	return err
}

// OpenExpiredVoting abre automaticamente a votação de partidas que já
// terminaram (fim previsto no passado, com folga de delayMin minutos) e ainda
// estão aguardando encerramento manual do admin — sem isso, a votação só
// existia se o admin clicasse em "encerrar", e os jogadores não conseguiam
// votar. Idempotente: só toca partidas em 'closed' ou 'teams_drawn'.
// votingClosesDays define por quantos dias a votação fica aberta a partir do
// fim da partida.
func (s *Store) OpenExpiredVoting(ctx context.Context, delayMin, votingClosesDays int) ([]string, error) {
	// match_date + end_time não têm fuso; a partida acontece em America/Sao_Paulo
	// (TZ do processo em produção). A conversão é feita com fuso fixo no SQL para
	// isolar a comparação da TZ da sessão do PostgreSQL (Etc/UTC no container).
	rows, err := s.pool.Query(ctx, `
		update matches m
		set status = 'voting',
		    finished_at = now(),
		    voting_closes_at = (m.match_date + m.end_time)::timestamp
		        at time zone 'America/Sao_Paulo'
		        + make_interval(days => $2)
		where m.status in ('closed', 'teams_drawn')
		  and (m.match_date + m.end_time)::timestamp
		      at time zone 'America/Sao_Paulo' <= now() - make_interval(mins => $1)
		returning m.id`, delayMin, votingClosesDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectIDs(rows)
}

// RemoveVote desfaz um voto do usuário (clique num jogador já selecionado).
func (s *Store) RemoveVote(ctx context.Context, matchID, voterID, category, candidateID string) error {
	_, err := s.pool.Exec(ctx, `
		delete from votes
		where match_id = $1 and voter_id = $2 and category = $3 and candidate_id = $4`,
		matchID, voterID, category, candidateID)
	return err
}

// MyVotes retorna categoria → candidatos votados pelo usuário nesta partida.
func (s *Store) MyVotes(ctx context.Context, matchID, voterID string) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx,
		`select category, candidate_id from votes where match_id = $1 and voter_id = $2 order by created_at`, matchID, voterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	votes := map[string][]string{}
	for rows.Next() {
		var category, candidate string
		if err := rows.Scan(&category, &candidate); err != nil {
			return nil, err
		}
		votes[category] = append(votes[category], candidate)
	}
	return votes, rows.Err()
}

// --- FUT-005: dashboard de votação ---

// VoteTally: linha agregada por candidato numa categoria da partida.
type VoteTally struct {
	Category    string `json:"category"`
	CandidateID string `json:"candidate_id"`
	Name        string `json:"name"`
	AvatarColor string `json:"avatar_color"`
	Votes       int    `json:"votes"`
}

// FlatVote: voto individual (auditável).
type FlatVote struct {
	VoterID       string    `json:"voter_id"`
	VoterName     string    `json:"voter_name"`
	Category      string    `json:"category"`
	CandidateID   string    `json:"candidate_id"`
	CandidateName string    `json:"candidate_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// MatchVoteSummary: participação de votação por partida (comparativo).
type MatchVoteSummary struct {
	MatchID   string `json:"match_id"`
	MatchDate string `json:"match_date"` // DD/MM
	Status    string `json:"status"`
	Voters    int    `json:"voters"`
	Votes     int    `json:"votes"`
}

func (s *Store) VoteTallyByMatch(ctx context.Context, matchID string) ([]VoteTally, error) {
	rows, err := s.pool.Query(ctx, `
		select cats.category, c.candidate_id, u.name, u.avatar_color, c.votes
		from (
			select 'top_scorer' as category, 1 as ord
			union all select 'worst_player', 2
			union all select 'best_goal', 3
		) cats
		cross join lateral (
			select v.candidate_id, count(*) as votes
			from votes v where v.match_id = $1 and v.category = cats.category
			group by v.candidate_id
		) c
		join users u on u.id = c.candidate_id
		order by cats.ord, c.votes desc, u.name`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VoteTally
	for rows.Next() {
		var t VoteTally
		if err := rows.Scan(&t.Category, &t.CandidateID, &t.Name, &t.AvatarColor, &t.Votes); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MatchesVotesFlat lista votos individuais; userOpcional filtra por votante.
func (s *Store) MatchesVotesFlat(ctx context.Context, matchID, userID string) ([]FlatVote, error) {
	q := `
		select v.voter_id, vu.name, v.category, v.candidate_id, cu.name, v.created_at
		from votes v
		join users vu on vu.id = v.voter_id
		join users cu on cu.id = v.candidate_id
		where v.match_id = $1`
	args := []any{matchID}
	if userID != "" {
		q += ` and v.voter_id = $2`
		args = append(args, userID)
	}
	q += ` order by v.created_at`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FlatVote
	for rows.Next() {
		var f FlatVote
		if err := rows.Scan(&f.VoterID, &f.VoterName, &f.Category, &f.CandidateID, &f.CandidateName, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UserVoteHistory — tudo que o usuário votou em qualquer partida.
func (s *Store) UserVoteHistory(ctx context.Context, userID string) ([]FlatVote, error) {
	rows, err := s.pool.Query(ctx, `
		select v.voter_id, vu.name, v.category, v.candidate_id, cu.name, v.created_at
		from votes v
		join users vu on vu.id = v.voter_id
		join users cu on cu.id = v.candidate_id
		where v.voter_id = $1
		order by v.created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FlatVote
	for rows.Next() {
		var f FlatVote
		if err := rows.Scan(&f.VoterID, &f.VoterName, &f.Category, &f.CandidateID, &f.CandidateName, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// MatchesVoteSummary — join para o comparativo entre partidas.
func (s *Store) MatchesVoteSummary(ctx context.Context, fromISO, toISO string) ([]MatchVoteSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select m.id, to_char(m.match_date, 'DD/MM'), m.status,
			coalesce(t.voters, 0), coalesce(t.votes, 0)
		from matches m
		left join lateral (
			select count(distinct v.voter_id) as voters, count(*) as votes
			from votes v where v.match_id = m.id
		) t on true
		where ($1 = '' or to_char(m.match_date, 'YYYY-MM-DD') >= $1)
		  and ($2 = '' or to_char(m.match_date, 'YYYY-MM-DD') <= $2)
		order by m.match_date desc`, fromISO, toISO)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchVoteSummary
	for rows.Next() {
		var s2 MatchVoteSummary
		if err := rows.Scan(&s2.MatchID, &s2.MatchDate, &s2.Status, &s2.Voters, &s2.Votes); err != nil {
			return nil, err
		}
		out = append(out, s2)
	}
	return out, rows.Err()
}
func (s *Store) VoteResults(ctx context.Context, matchID string) ([]VoteResult, error) {
	results := []VoteResult{
		{Category: "top_scorer", Winners: []Winner{}},
		{Category: "worst_player", Winners: []Winner{}},
	}
	for i := range results {
		rows, err := s.pool.Query(ctx, `
			with tallies as (
				select v.candidate_id, count(*) as votes
				from votes v where v.match_id = $1 and v.category = $2
				group by v.candidate_id
			)
			select t.candidate_id, u.name, u.avatar_color, t.votes,
			       (select coalesce(sum(votes), 0) from tallies) as total
			from tallies t join users u on u.id = t.candidate_id
			where t.votes = (select max(votes) from tallies)
			order by u.name`, matchID, results[i].Category)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var w Winner
			var total int
			if err := rows.Scan(&w.UserID, &w.Name, &w.AvatarColor, &w.Votes, &total); err != nil {
				rows.Close()
				return nil, err
			}
			results[i].VoteCount = total
			results[i].Winners = append(results[i].Winners, w)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return results, nil
}
