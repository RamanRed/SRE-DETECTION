DROP INDEX IF EXISTS uq_pipeline_external_build;

CREATE UNIQUE INDEX uq_pipeline_external_build
    ON pipeline_builds(ci_tool, pipeline_name, external_build_id)
    WHERE external_build_id IS NOT NULL;
