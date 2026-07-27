FROM node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY . .
ARG VITE_AUTH_MODE
ARG VITE_OIDC_PROVIDER
ARG VITE_OIDC_AUTHORITY
ARG VITE_OIDC_CLIENT_ID

ENV VITE_AUTH_MODE=$VITE_AUTH_MODE
ENV VITE_OIDC_PROVIDER=$VITE_OIDC_PROVIDER
ENV VITE_OIDC_AUTHORITY=$VITE_OIDC_AUTHORITY
ENV VITE_OIDC_CLIENT_ID=$VITE_OIDC_CLIENT_ID

RUN npx vite build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
