CREATE TABLE IF NOT EXISTS property_leads (
  id VARCHAR(36) PRIMARY KEY,
  source_name VARCHAR(128) NOT NULL DEFAULT '',
  source_phone VARCHAR(32) NOT NULL DEFAULT '',
  community VARCHAR(200) NOT NULL,
  location VARCHAR(200) NOT NULL DEFAULT '',
  layout VARCHAR(100) NOT NULL DEFAULT '',
  expected_rent_monthly BIGINT NOT NULL DEFAULT 0,
  note TEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'new',
  converted_property_id VARCHAR(36) NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_property_leads_status (status),
  INDEX idx_property_leads_phone (source_phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS property_status_history (
  id VARCHAR(36) PRIMARY KEY,
  property_id VARCHAR(36) NOT NULL,
  from_status VARCHAR(32) NOT NULL DEFAULT '',
  to_status VARCHAR(32) NOT NULL,
  operator_id VARCHAR(36) NOT NULL DEFAULT '',
  note VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_property_status_history_property (property_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
