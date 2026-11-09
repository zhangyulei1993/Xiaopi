ALTER TABLE properties
  ADD COLUMN actual_layout VARCHAR(100) NOT NULL DEFAULT '' AFTER layout,
  ADD COLUMN actual_area_sqm INT NOT NULL DEFAULT 0 AFTER area_sqm;
