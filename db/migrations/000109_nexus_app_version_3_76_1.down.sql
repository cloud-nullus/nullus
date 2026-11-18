-- 000109 를 되돌린다 — Nexus 항목의 app_version 3.76.1 → 3.64.0.
UPDATE golden_path_templates t
   SET tools = (
           SELECT jsonb_agg(
                      CASE
                          WHEN elem.value->>'name' = 'Nexus' AND elem.value->>'app_version' = '3.76.1'
                              THEN elem.value || jsonb_build_object('app_version', '3.64.0')
                          ELSE elem.value
                      END
                      ORDER BY elem.ordinality)
             FROM jsonb_array_elements(t.tools) WITH ORDINALITY AS elem(value, ordinality)
       ),
       updated_at = NOW()
 WHERE jsonb_typeof(t.tools) = 'array'
   AND EXISTS (
           SELECT 1
             FROM jsonb_array_elements(t.tools) AS elem
            WHERE elem->>'name' = 'Nexus' AND elem->>'app_version' = '3.76.1'
       );

UPDATE compatibility_matrices m
   SET tools = (
           SELECT jsonb_object_agg(
                      e.key,
                      CASE
                          WHEN e.value->>'Name' = 'Nexus' AND e.value->>'AppVersion' = '3.76.1'
                              THEN e.value || jsonb_build_object('AppVersion', '3.64.0')
                          ELSE e.value
                      END)
             FROM jsonb_each(m.tools) AS e
       ),
       updated_at = NOW()
 WHERE jsonb_typeof(m.tools) = 'object'
   AND EXISTS (
           SELECT 1
             FROM jsonb_each(m.tools) AS e
            WHERE e.value->>'Name' = 'Nexus' AND e.value->>'AppVersion' = '3.76.1'
       );
