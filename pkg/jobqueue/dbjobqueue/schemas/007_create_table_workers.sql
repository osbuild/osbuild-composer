CREATE TABLE workers(
       worker_id uuid PRIMARY KEY,
       arch varchar NOT NULL,
       heartbeat timestamp NOT NULL
);

ALTER TABLE heartbeats
ADD COLUMN worker_id uuid REFERENCES workers(worker_id) ON DELETE CASCADE;
