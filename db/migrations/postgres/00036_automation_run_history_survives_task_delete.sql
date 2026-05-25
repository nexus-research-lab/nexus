-- +goose Up
DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT con.conname
    INTO constraint_name
    FROM pg_constraint AS con
    JOIN pg_class AS rel ON rel.oid = con.conrelid
    JOIN pg_class AS ref ON ref.oid = con.confrelid
    JOIN pg_attribute AS att ON att.attrelid = rel.oid AND att.attnum = ANY(con.conkey)
    WHERE con.contype = 'f'
      AND rel.relname = 'automation_cron_runs'
      AND ref.relname = 'automation_cron_jobs'
      AND att.attname = 'job_id'
    LIMIT 1;

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE automation_cron_runs DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

-- +goose Down
DELETE FROM automation_cron_runs AS run
WHERE NOT EXISTS (
    SELECT 1
    FROM automation_cron_jobs AS job
    WHERE job.job_id = run.job_id
);

DO $$
DECLARE
    constraint_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1
        FROM pg_constraint AS con
        JOIN pg_class AS rel ON rel.oid = con.conrelid
        JOIN pg_class AS ref ON ref.oid = con.confrelid
        JOIN pg_attribute AS att ON att.attrelid = rel.oid AND att.attnum = ANY(con.conkey)
        WHERE con.contype = 'f'
          AND rel.relname = 'automation_cron_runs'
          AND ref.relname = 'automation_cron_jobs'
          AND att.attname = 'job_id'
    )
    INTO constraint_exists;

    IF NOT constraint_exists THEN
        ALTER TABLE automation_cron_runs
            ADD CONSTRAINT automation_cron_runs_job_id_fkey
            FOREIGN KEY (job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE;
    END IF;
END $$;
