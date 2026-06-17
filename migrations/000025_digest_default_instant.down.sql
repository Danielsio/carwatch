-- Restore the 000024 behavior: new users default to 'digest'.
ALTER TABLE users ALTER COLUMN digest_mode SET DEFAULT 'digest';
