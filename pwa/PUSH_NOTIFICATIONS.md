# Notificações de partidas

A ativação fica no Perfil e é individual por aparelho. O service worker recebe Web Push com o app fechado e abre `/partida?match=<id>` quando o jogador toca no aviso. Essa tela carrega a partida indicada, e o login preserva esse destino caso a sessão expire.

O navegador solicita permissão apenas após o jogador tocar em Ativar. No iPhone, primeiro adicionar o app à tela inicial pelo Safari. Em aparelhos sem suporte, o Perfil mostra orientações; uma permissão bloqueada precisa ser liberada nas configurações do aparelho/navegador.

O frontend consulta a chave pública em `/api/push/config` e não recebe a chave privada. Publique a API antes da PWA; a preparação do Compose e das chaves VAPID está documentada em `fut-app-api/PUSH_NOTIFICATIONS.md` no repositório da API.

## Verificar

```bash
npm test
npm run build
npm run preview -- --host 127.0.0.1
```

Use o preview do build para testar o service worker em localhost. O servidor Vite de desenvolvimento não registra o worker de produção. O preview não inclui o proxy da API por padrão; este projeto configura `preview.proxy` para acessar a API local em 8080.

Validação em celular exige o domínio HTTPS publicado. Os testes automatizados simulam as APIs de push e não enviam avisos reais.
