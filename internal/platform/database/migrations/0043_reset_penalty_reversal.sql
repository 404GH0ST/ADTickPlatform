-- A prepared participant reset charges before Docker can remove the volume.
-- Keep enough provenance to reverse that charge only when the controller
-- confirms that it failed before touching the volume.
ALTER TABLE flag_retention_failures
    ADD COLUMN reset_attempt_token TEXT,
    ADD COLUMN reset_created BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN reset_prior_confirmed BOOLEAN,
    ADD COLUMN reset_prior_penalty DOUBLE PRECISION;
