ALTER TABLE challenges
  ADD COLUMN IF NOT EXISTS last_validation_status TEXT,
  ADD COLUMN IF NOT EXISTS last_validation_baseline_ssh_contract_ok BOOLEAN,
  ADD COLUMN IF NOT EXISTS last_validation_checker_contract_ok BOOLEAN,
  ADD COLUMN IF NOT EXISTS last_validation_service_state_contract_ok BOOLEAN,
  ADD COLUMN IF NOT EXISTS last_validation_checked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_validation_message TEXT;
