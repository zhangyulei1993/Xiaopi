CREATE TABLE IF NOT EXISTS subscription_outbox (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  kind VARCHAR(16) NOT NULL,
  subject_id VARCHAR(36) NOT NULL,
  payload JSON NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  next_attempt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at TIMESTAMP NULL,
  INDEX idx_subscription_outbox_due (finished_at, next_attempt)
);

CREATE TABLE IF NOT EXISTS subscription_deliveries (
  job_id BIGINT UNSIGNED NOT NULL,
  user_id VARCHAR(36) NOT NULL,
  PRIMARY KEY (job_id,user_id)
);
