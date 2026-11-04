ALTER TABLE properties
  ADD COLUMN rent_type VARCHAR(16) NOT NULL DEFAULT '' AFTER fee_text,
  ADD COLUMN available_date VARCHAR(32) NOT NULL DEFAULT '' AFTER rent_type,
  ADD COLUMN deposit_payment VARCHAR(64) NOT NULL DEFAULT '' AFTER available_date,
  ADD COLUMN utilities_text VARCHAR(64) NOT NULL DEFAULT '' AFTER deposit_payment,
  ADD COLUMN area_sqm INT NOT NULL DEFAULT 0 AFTER utilities_text,
  ADD COLUMN floor_info VARCHAR(64) NOT NULL DEFAULT '' AFTER area_sqm,
  ADD COLUMN orientation VARCHAR(32) NOT NULL DEFAULT '' AFTER floor_info,
  ADD COLUMN elevator VARCHAR(32) NOT NULL DEFAULT '' AFTER orientation,
  ADD COLUMN metro_minutes INT NOT NULL DEFAULT 0 AFTER elevator,
  ADD COLUMN verification_date VARCHAR(32) NOT NULL DEFAULT '' AFTER metro_minutes,
  ADD COLUMN rules_text VARCHAR(200) NOT NULL DEFAULT '' AFTER verification_date;
