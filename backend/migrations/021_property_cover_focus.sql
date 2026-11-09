ALTER TABLE properties
  ADD COLUMN cover_focus_x TINYINT NOT NULL DEFAULT 50 AFTER area_label,
  ADD COLUMN cover_focus_y TINYINT NOT NULL DEFAULT 50 AFTER cover_focus_x;
