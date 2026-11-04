CREATE TABLE IF NOT EXISTS property_favorites (
  user_id VARCHAR(36) NOT NULL,
  property_id VARCHAR(36) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, property_id),
  INDEX idx_property_favorites_property (property_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS rental_requests (
  id VARCHAR(36) PRIMARY KEY,
  user_id VARCHAR(36) NOT NULL,
  request_type VARCHAR(32) NOT NULL DEFAULT 'general',
  property_id VARCHAR(36) NOT NULL DEFAULT '',
  phone VARCHAR(32) NOT NULL,
  location VARCHAR(200) NOT NULL DEFAULT '',
  layout VARCHAR(100) NOT NULL DEFAULT '',
  budget_min BIGINT NOT NULL DEFAULT 0,
  budget_max BIGINT NOT NULL DEFAULT 0,
  move_in_date VARCHAR(32) NOT NULL DEFAULT '',
  note TEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'new',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_rental_requests_status (status),
  INDEX idx_rental_requests_user (user_id),
  INDEX idx_rental_requests_property (property_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
