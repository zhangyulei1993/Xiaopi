CREATE TABLE IF NOT EXISTS user_footprints (
  user_id VARCHAR(36) NOT NULL,
  property_id VARCHAR(36) NOT NULL,
  viewed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  view_count INT UNSIGNED NOT NULL DEFAULT 1,
  PRIMARY KEY (user_id, property_id),
  INDEX idx_user_footprints_viewed (user_id, viewed_at)
);
