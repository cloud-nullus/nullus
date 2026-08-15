DELETE FROM compatibility_matrices WHERE id = 'gitea-jenkins-argocd-lite-v1';
DELETE FROM golden_path_templates WHERE id = 'gitea-jenkins-argocd-lite-v1';

ALTER TABLE golden_path_templates
    DROP CONSTRAINT IF EXISTS golden_path_templates_planning_profile_check;

ALTER TABLE golden_path_templates
    DROP COLUMN IF EXISTS planning_profile;
