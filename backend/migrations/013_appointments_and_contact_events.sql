ALTER TABLE rental_requests
  ADD COLUMN contact_name VARCHAR(80) NOT NULL DEFAULT '' AFTER property_id,
  ADD COLUMN preferred_time VARCHAR(80) NOT NULL DEFAULT '' AFTER phone;

CREATE TABLE IF NOT EXISTS property_contact_events (
  id VARCHAR(36) PRIMARY KEY,
  user_id VARCHAR(36) NOT NULL,
  property_id VARCHAR(36) NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  source VARCHAR(32) NOT NULL DEFAULT 'detail',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_contact_events_property (property_id, created_at),
  INDEX idx_contact_events_user (user_id, created_at),
  INDEX idx_contact_events_dedupe (user_id, property_id, event_type, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
