FROM --platform=$BUILDPLATFORM node:22-alpine AS builder
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
# 기동 시 /config.js 를 환경변수로 만들어, OIDC 설정을 재빌드 없이 바꿀 수 있게 한다.
# 위 ARG/ENV 는 이제 "기본값"일 뿐이고 실제 배포 값은 런타임에 들어온다.
COPY docker-entrypoint.d/40-nullus-runtime-config.sh /docker-entrypoint.d/40-nullus-runtime-config.sh
RUN chmod +x /docker-entrypoint.d/40-nullus-runtime-config.sh
EXPOSE 8080
