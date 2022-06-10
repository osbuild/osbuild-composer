ALTER TABLE job_dependencies

DROP CONSTRAINT job_dependencies_dependency_id_fkey,

DROP CONSTRAINT job_dependencies_job_id_fkey,

ADD CONSTRAINT job_dependencies_dependency_id_fkey
FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,

ADD CONSTRAINT job_dependencies_job_id_fkey
FOREIGN KEY (dependency_id) REFERENCES jobs(id) ON DELETE CASCADE;
