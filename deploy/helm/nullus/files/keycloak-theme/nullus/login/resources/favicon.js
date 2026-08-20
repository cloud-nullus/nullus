// 파비콘을 Nullus 마크로 바꾼다.
//
// 부모 템플릿(keycloak.v2)이 <link rel="icon" href="${resourcesPath}/img/favicon.ico">
// 를 박아 넣는다. 우리 테마에 그 파일이 없으면 Keycloak 이 부모 테마의 것으로
// 폴백해서 키클록 로고가 뜬다.
//
// resources/img/favicon.ico 를 만들어 덮어쓸 수도 있다. 그러려면 resources 아래에
// 폴더가 하나 더 생기고, 쿠버네티스에서는 ConfigMap 볼륨을 중첩해서 마운트해야
// 한다 — theme.properties 가 리소스를 한 단으로 두기로 한 이유가 그것이다.
// 게다가 .ico 는 바이너리라 ConfigMap 의 data(문자열)로는 못 싣는다.
// 링크 한 줄을 바꾸는 편이 훨씬 싸다.
//
// 이 파일이 막히면 키클록 기본 파비콘이 그대로 남을 뿐, 화면은 멀쩡하다.
(function () {
  // 리소스 경로는 이 스크립트 자신의 주소에서 얻는다 — Keycloak 이 경로에 버전
  // 해시를 넣어서 미리 적어 둘 수 없다. currentScript 는 지금 이 순간에만
  // 읽히므로 먼저 붙잡아 둔다.
  var src = (document.currentScript || {}).src;
  if (!src) return;

  var link = document.createElement("link");
  link.rel = "icon";
  link.type = "image/svg+xml";
  link.href = src.replace(/\/[^/]*$/, "/knot.svg");

  // href 만 갈아 끼우면 이미 받은 아이콘을 그대로 쓰는 브라우저가 있다.
  // 노드를 통째로 바꿔야 다시 받는다.
  var old = document.querySelector('link[rel~="icon"]');
  if (old && old.parentNode) old.parentNode.replaceChild(link, old);
  else document.head.appendChild(link);
})();
