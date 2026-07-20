# Só constrói os assets estáticos: o Caddy é quem os serve.
FROM node:24-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM alpine:3.21
COPY --from=build /src/dist /dist
