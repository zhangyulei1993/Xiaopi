ALTER TABLE properties
  ADD COLUMN converted_property_id VARCHAR(64) NOT NULL DEFAULT '' AFTER material_category;

CREATE INDEX idx_properties_converted_property_id ON properties (converted_property_id);
