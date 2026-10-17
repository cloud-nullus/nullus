-- metrics-server 를 스택 네임스페이스에 깔지 않는다.
--
-- 000056 시드는 이 행의 namespace 를 NULL 로 두었다. chartSpecForStep 은 DB
-- 메타데이터를 코드 기본값보다 먼저 보므로, helm_step_metadata.go 가 못박은
-- kube-system 이 실제 설치에서 무시되고 애드온이 스택 네임스페이스로 들어갔다.
--
-- 그러면 이 차트가 만드는 APIService v1beta1.metrics.k8s.io 가 그 네임스페이스의
-- Service 를 가리킨다. 스택을 지우면 Service 만 사라지고 APIService 는 죽은 대상을
-- 계속 가리켜 API discovery 가 실패하고, 그 클러스터의 **모든** 네임스페이스 삭제가
-- 교착된다("metrics.k8s.io/v1beta1: stale GroupVersion discovery").
--
-- cert-manager 행은 처음부터 'cert-manager' 로 못박혀 있어 이 문제를 피했다.
UPDATE stack_helm_step_configs
   SET namespace  = 'kube-system',
       updated_at = NOW()
 WHERE step_name  = 'installing_metrics_server'
   AND (namespace IS NULL OR namespace = '');
