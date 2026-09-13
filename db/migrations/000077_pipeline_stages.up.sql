-- 000077_pipeline_stages.up.sql
-- 파이프라인이 실제로 가진 단계를 파이프라인 단위로 둔다.
--
-- 템플릿의 stages 는 템플릿 단위다. 이미지 스캔 같은 선택 단계는 파이프라인마다
-- 켜고 끄므로 템플릿이 담을 수 없다 — 템플릿에 두면 끈 파이프라인이 돌지도 않은
-- 단계를 보여준다(000070 이 되돌린 실패와 같은 모양).
--
-- 기존 파이프라인은 빈 배열로 둔다. 무엇을 스캐폴딩했는지 기록이 없으므로
-- 지어내지 않는다 — 화면은 빈 배열을 "모름" 으로 보고 템플릿으로 떨어진다.
ALTER TABLE pipelines
    ADD COLUMN IF NOT EXISTS stages JSONB NOT NULL DEFAULT '[]'::jsonb;
