ALTER TABLE property_leads
  ADD COLUMN follow_up_note VARCHAR(500) NOT NULL DEFAULT '' AFTER note;

ALTER TABLE rental_requests
  ADD COLUMN follow_up_note VARCHAR(500) NOT NULL DEFAULT '' AFTER note,
  ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER created_at;

CREATE TABLE IF NOT EXISTS operation_logs (
  id VARCHAR(36) PRIMARY KEY,
  operator_id VARCHAR(36) NOT NULL DEFAULT '',
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(36) NOT NULL,
  action VARCHAR(64) NOT NULL,
  detail VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_operation_logs_created (created_at),
  INDEX idx_operation_logs_target (target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
