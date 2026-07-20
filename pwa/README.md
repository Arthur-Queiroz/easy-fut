# Fut App PWA

PWA Vue do Fut App. Em producao, o Caddy desta aplicacao entrega os arquivos estaticos e encaminha `/api` para o container `api` na rede Docker externa `fut-app`.

## Desenvolvimento

```bash
npm ci
npm run dev
```

Para executar o Compose localmente, crie a rede uma vez:

```bash
docker network create fut-app
docker compose up --build
```

## Verificacao

```bash
npm run build
```
