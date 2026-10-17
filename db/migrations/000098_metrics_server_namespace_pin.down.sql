-- 되돌리면 metrics-server 가 다시 스택 네임스페이스로 설치된다(네임스페이스 삭제 교착).
UPDATE stack_helm_step_configs
   SET namespace  = NULL,
       updated_at = NOW()
 WHERE step_name  = 'installing_metrics_server'
   AND namespace  = 'kube-system';
