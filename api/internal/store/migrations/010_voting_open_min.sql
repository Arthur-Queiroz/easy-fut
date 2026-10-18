-- FUT-001: delay de abertura automática da votação (minutos após o fim da
-- partida). Default 0 = a votação abre no primeiro tick do cron após o fim.
insert into settings (key, value) values ('voting_open_delay_min', '0')
on conflict (key) do nothing;
