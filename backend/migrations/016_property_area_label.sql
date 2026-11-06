ALTER TABLE properties
  ADD COLUMN area_label VARCHAR(64) NOT NULL DEFAULT '' AFTER location;
