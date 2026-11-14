CREATE TABLE IF NOT EXISTS web_admin_accounts (
  username VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
  password_hash VARCHAR(160) NOT NULL,
  session_version CHAR(36) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE
);
