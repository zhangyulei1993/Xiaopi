ALTER TABLE properties
  ADD COLUMN is_material TINYINT(1) NOT NULL DEFAULT 0 AFTER display_tags,
  ADD COLUMN material_category VARCHAR(80) NOT NULL DEFAULT '' AFTER is_material;
