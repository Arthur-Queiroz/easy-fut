-- FUT-004: a materialização convertia dia + horário do template no fuso da
-- sessão do PostgreSQL (UTC no container), gravando o prazo de confirmação
-- 3 h antes do horário de São Paulo. Corrige as partidas ainda abertas cujo
-- prazo é exatamente o valor gerado pelo bug; prazo editado pelo admin e
-- partida já fechada ficam como estão. Com a sessão já em São Paulo, os dois
-- valores coincidem e nada muda.
update matches m
set confirmation_deadline = case
		when r.same_day_deadline_time is not null
			then (m.match_date + r.same_day_deadline_time) at time zone 'America/Sao_Paulo'
		else ((m.match_date + r.start_time) at time zone 'America/Sao_Paulo') - interval '30 minutes'
	end
from recurring_matches r
where r.id = m.recurring_id
  and m.status = 'open'
  and m.confirmation_deadline = case
		when r.same_day_deadline_time is not null
			then (m.match_date + r.same_day_deadline_time)::timestamptz
		else (m.match_date + r.start_time)::timestamptz - interval '30 minutes'
	end;
