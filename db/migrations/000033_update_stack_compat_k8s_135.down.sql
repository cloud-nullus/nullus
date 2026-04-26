UPDATE compatibility_matrices
SET k8s_max = CASE
      WHEN id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1', 'github-argocd-v1') THEN '1.32'
      ELSE k8s_max
    END,
    k8s_recommended = CASE
      WHEN id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1') THEN '1.30'
      WHEN id = 'github-argocd-v1' THEN '1.29'
      ELSE k8s_recommended
    END,
    updated_at = NOW()
WHERE id IN ('gitlab-allinone-v1', 'gitlab-argocd-v1', 'github-argocd-v1');
