ALTER TABLE properties
  ADD COLUMN building_no VARCHAR(32) NOT NULL DEFAULT '' AFTER location,
  ADD COLUMN room_no VARCHAR(32) NOT NULL DEFAULT '' AFTER building_no;
