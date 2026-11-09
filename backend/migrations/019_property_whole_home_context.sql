ALTER TABLE properties
  ADD COLUMN whole_layout VARCHAR(100) NOT NULL DEFAULT '' AFTER actual_layout,
  ADD COLUMN whole_area_sqm INT NOT NULL DEFAULT 0 AFTER actual_area_sqm;
