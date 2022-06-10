ALTER TABLE heartbeats

DROP CONSTRAINT heartbeats_id_fkey,

ADD CONSTRAINT heartbeats_id_fkey
FOREIGN KEY (id) REFERENCES jobs(id) ON DELETE CASCADE;
