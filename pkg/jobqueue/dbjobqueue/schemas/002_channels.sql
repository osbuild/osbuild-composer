ALTER TABLE jobs
ADD COLUMN channel varchar NOT NULL DEFAULT '';

-- We added a column, thus we have to recreate the view.
CREATE OR REPLACE VIEW ready_jobs AS
SELECT *
FROM jobs
WHERE started_at IS NULL
  AND canceled = FALSE
  AND id NOT IN (
    SELECT job_id
    FROM job_dependencies JOIN jobs ON dependency_id = id
    WHERE finished_at IS NULL
)
ORDER BY queued_at ASC
