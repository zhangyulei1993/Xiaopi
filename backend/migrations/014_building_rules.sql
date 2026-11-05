ALTER TABLE properties ADD COLUMN unit_no VARCHAR(32) NOT NULL DEFAULT '' AFTER building_no;

CREATE TABLE IF NOT EXISTS building_rules (
  id VARCHAR(36) PRIMARY KEY,
  community VARCHAR(200) NOT NULL,
  building_no VARCHAR(32) NOT NULL,
  unit_count INT NOT NULL DEFAULT 1,
  floor_start INT NOT NULL DEFAULT 1,
  floor_count INT NOT NULL,
  households_per_floor INT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_building_rule (community, building_no),
  INDEX idx_building_rules_community (community)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
