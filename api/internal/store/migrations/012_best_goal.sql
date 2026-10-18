-- FUT-005: categoria "best_goal" (melhor gol) entra no check da tabela votes.
alter table votes drop constraint if exists votes_category_check;
alter table votes add constraint votes_category_check
  check (category in ('top_scorer', 'worst_player', 'best_goal'));
