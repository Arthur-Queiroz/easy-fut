FROM node:24-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# A imagem publicada precisa servir o site sozinha: o deployctl da VPS sobe
# este container e espera um servidor HTTP na porta 80 (manifesto fut-app,
# servico "web"). Uma imagem so com os assets sai imediatamente e o release
# falha ao conectar o container na rede.
FROM caddy:2-alpine
COPY --from=build /src/dist /srv
COPY Caddyfile /etc/caddy/Caddyfile

# O container roda com --read-only; /tmp e o unico ponto gravavel (tmpfs).
ENV XDG_CONFIG_HOME=/tmp \
    XDG_DATA_HOME=/tmp

EXPOSE 80
