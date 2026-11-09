ALTER TABLE properties
  ADD COLUMN marketing_phrase VARCHAR(64) NOT NULL DEFAULT '' AFTER title;
