UPDATE compatibility_matrices
SET k8s_max = '1.35',
    k8s_recommended = '1.35',
    updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1', 'github-argocd-v1');
