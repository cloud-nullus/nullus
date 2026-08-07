// 로컬 개발용 기본 런타임 설정 — 비워 두면 빌드 시 값(VITE_*)으로 폴백한다.
// 컨테이너에서는 기동 시 이 파일이 환경변수 기반으로 덮어써진다
// (web/docker-entrypoint.d/40-nullus-runtime-config.sh).
window.__NULLUS_CONFIG__ = {}
