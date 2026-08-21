package scaffold

import (
	"fmt"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// React 스캐폴딩이 쓰는 버전들.
//
// 플랫폼 자신이 쓰는 조합(web/package.json)에 맞춘다. 함께 도는 것이 이미
// 검증됐고, 팀이 아는 스택과 어긋나지 않는다. "최신" 을 따로 추적하면 서로
// 맞지 않는 조합을 사용자에게 떠넘기게 된다.
const (
	reactVersion      = "^19.2.4"
	viteVersion       = "^8.0.0"
	vitePluginVersion = "^6.0.0"
	typescriptVersion = "~5.9.3"
	reactTypesVersion = "^19.2.0"
	nodeBuilderImage  = "node:22-alpine"
)

// reactAppFiles 는 바로 도는 React + Vite + TypeScript 앱을 만든다.
//
// 예전에는 앱 타입과 무관하게 nginx 기본 페이지를 담은 Dockerfile 하나만
// 만들었다. 배포는 성공하는데 열어 보면 "Welcome to nginx" 가 떴고, 개발자는
// 그제서야 그 안에 앱이 없다는 것을 알게 된다.
func reactAppFiles(app string, appPort int32) []port.CommitFile {
	return []port.CommitFile{
		{Path: "package.json", Content: reactPackageJSON(app)},
		{Path: "tsconfig.json", Content: reactTSConfig()},
		{Path: "vite.config.ts", Content: reactViteConfig()},
		{Path: "index.html", Content: reactIndexHTML(app)},
		{Path: "src/main.tsx", Content: reactMainTSX()},
		{Path: "src/App.tsx", Content: reactAppTSX(app)},
		{Path: ".dockerignore", Content: reactDockerignore()},
	}
}

func reactPackageJSON(app string) string {
	return fmt.Sprintf(`{
  "name": %q,
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": %q,
    "react-dom": %q
  },
  "devDependencies": {
    "@types/react": %q,
    "@types/react-dom": %q,
    "@vitejs/plugin-react": %q,
    "typescript": %q,
    "vite": %q
  }
}
`, app, reactVersion, reactVersion,
		reactTypesVersion, reactTypesVersion, vitePluginVersion,
		typescriptVersion, viteVersion)
}

func reactTSConfig() string {
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true
  },
  "include": ["src"]
}
`
}

func reactViteConfig() string {
	return `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
})
`
}

func reactIndexHTML(app string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="ko">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`, app)
}

func reactMainTSX() string {
	return `import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import App from './App'

const container = document.getElementById('root')
if (!container) {
  throw new Error('#root 를 찾지 못했습니다')
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
`
}

func reactAppTSX(app string) string {
	return fmt.Sprintf(`export default function App() {
  return (
    <main style={{ fontFamily: 'system-ui, sans-serif', padding: '3rem' }}>
      <h1>%s</h1>
      <p>Nullus 가 만든 React 앱입니다. src/App.tsx 부터 고쳐 나가세요.</p>
    </main>
  )
}
`, app)
}

func reactDockerignore() string {
	// node_modules 를 그대로 실으면 컨텍스트가 수백 MB 가 되고, 빌더가 로컬
	// 플랫폼에 맞춰 설치된 바이너리를 그대로 쓰게 된다.
	return `node_modules
dist
.git
`
}

// reactDockerfile 은 빌드와 서빙을 나눈 2단계 이미지다.
//
// node 이미지를 그대로 배포하면 개발 서버가 프로덕션에 뜨고 이미지도 수백 MB 가
// 된다. 빌드 결과만 nginx 로 옮긴다.
func reactDockerfile(appPort int32) string {
	return fmt.Sprintf(`# Nullus 가 생성한 Dockerfile 입니다.
# 빌드와 서빙을 나눕니다 — 최종 이미지에는 빌드 결과만 담깁니다.
FROM %s AS build
WORKDIR /app
# 의존성 먼저 복사해 소스만 바뀐 빌드에서 레이어 캐시를 살린다.
COPY package.json ./
# 첫 스캐폴딩에는 lock 파일이 없으므로 npm ci 를 쓰지 않는다.
RUN npm install
COPY . .
RUN npm run build

FROM nginx:alpine
# SPA 는 새로고침하면 서버가 그 경로를 찾는다. try_files 로 index.html 에
# 되돌리지 않으면 첫 화면 말고는 전부 404 가 된다.
#
# 포트는 배포 매니페스트가 가리키는 값으로 맞춘다. 기본 80 으로 두면 Service 의
# targetPort 와 어긋나 endpoints 가 비고, 원인이 멀리 떨어진 503 이 된다.
RUN printf 'server {\n    listen %d;\n    location / {\n        root  /usr/share/nginx/html;\n        index index.html;\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE %d
`, nodeBuilderImage, appPort, appPort)
}
