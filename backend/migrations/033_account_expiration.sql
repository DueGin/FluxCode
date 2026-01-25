-- 033_account_expiration.sql
-- 为账号增加过期时间字段，并加速相应的调度查询

ALTER TABLE accounts
	ADD COLUMN IF NOT EXISTS expires_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_accounts_expires_at
	ON accounts(expires_at)
	WHERE deleted_at IS NULL;

COMMENT ON COLUMN accounts.expires_at IS '账号过期时间，超过该时间后视为不可调度';
