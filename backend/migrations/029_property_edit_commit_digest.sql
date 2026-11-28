ALTER TABLE properties
  ADD COLUMN last_edit_digest CHAR(64) NOT NULL DEFAULT '';
