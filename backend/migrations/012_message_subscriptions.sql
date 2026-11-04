CREATE TABLE IF NOT EXISTS message_subscription_grants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_id VARCHAR(36) NOT NULL,
  template_id VARCHAR(128) NOT NULL,
  area VARCHAR(100) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  used_at TIMESTAMP NULL DEFAULT NULL,
  INDEX idx_message_grants_lookup (template_id, status, area, created_at),
  INDEX idx_message_grants_user (user_id, status)
);

ALTER TABLE rental_requests ADD COLUMN admin_notified_at TIMESTAMP NULL DEFAULT NULL;
