-- Per-challenge unlock proof epoch. Bumping invalidates previously issued
-- proofs so teams must re-extract after a security fix redeploy. Epoch 1 is
-- the pre-epoch default (MAC payload stays compatible with legacy proofs).
ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS unlock_proof_epoch INTEGER NOT NULL DEFAULT 1;

ALTER TABLE challenges
    ADD CONSTRAINT challenges_unlock_proof_epoch_positive
    CHECK (unlock_proof_epoch >= 1);
