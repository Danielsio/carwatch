-- Revert the new-user delivery default from 'digest' (set in 000024) back to
-- 'instant'. New users should receive immediate per-match Telegram alerts;
-- digest remains opt-in. Existing users keep whatever mode they already chose.
ALTER TABLE users ALTER COLUMN digest_mode SET DEFAULT 'instant';
