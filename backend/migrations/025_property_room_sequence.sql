ALTER TABLE properties
  ADD COLUMN room_sequence INT NOT NULL DEFAULT 0 AFTER whole_layout;
