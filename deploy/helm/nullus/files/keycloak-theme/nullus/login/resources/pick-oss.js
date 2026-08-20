// 자동 생성 — scripts/emit-keycloak-theme-art.py. 손으로 고치지 않는다.
//
// 왼쪽 벨트에 실릴 OSS 셋을 카탈로그에서 골라 CSS 변수에 꽂는다. 순수 CSS 로는
// 난수를 만들 수 없어 이 한 조각만 자바스크립트다. 이 파일이 막혀도 화면은
// scene.generated.css 의 기본 셋으로 그대로 뜬다.
(function () {
  var TOOLS = ["gitlab", "gitea", "jenkins", "argo", "harbor", "jfrog", "sonatype", "minio", "openbao", "prometheus", "grafana", "elasticsearch", "opensearch", "jaeger", "opentelemetry", "victoriametrics"];

  // 리소스 경로는 이 스크립트 자신의 주소에서 얻는다 — Keycloak 이 경로에 버전
  // 해시를 넣어서 미리 적어 둘 수 없다. currentScript 는 지금 이 순간에만
  // 읽히므로 먼저 붙잡아 둔다.
  var src = (document.currentScript || {}).src;

  function apply() {
    try {
      if (!src) return;
      var base = src.slice(0, src.lastIndexOf("/") + 1);
      var scene = document.querySelector(".nl-scene");
      if (!scene) return;

      var pool = TOOLS.slice();
      for (var i = pool.length - 1; i > 0; i--) {
        var j = Math.floor(Math.random() * (i + 1));
        var t = pool[i]; pool[i] = pool[j]; pool[j] = t;
      }
      ["a", "b", "c"].forEach(function (slot, i) {
        scene.style.setProperty("--nl-oss-" + slot,
          'url("' + base + "oss-" + pool[i] + '.svg")');
      });
    } catch (e) {
      // 고르기에 실패해도 기본 셋이 남아 있으므로 조용히 넘어간다.
    }
  }

  function start() {
    apply();
    // 한 바퀴가 넘어갈 때마다 다시 고른다. 그 순간에는 이 셋 중 어느 것도 화면에
    // 없다 — 크레인이 짐을 드는 시점이 경계보다 뒤이고 벨트 위 상자는 그 전에
    // 사라지도록 안무를 맞춰 두었다(생성기가 단언으로 지킨다). 그래서 새로고침
    // 없이도 바뀌는 게 눈에 띄지 않는다.
    var run = document.querySelector(".nl-run--in");
    if (!run) return;
    run.addEventListener("animationiteration", function (e) {
      // 안에 실린 조각들의 같은 이벤트가 올라온다. 한 바퀴에 한 번만 고른다.
      if (e.target === run) apply();
    });
  }

  // 이 스크립트는 <head> 에서 실행된다. 그때는 아직 장면이 없다.
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
