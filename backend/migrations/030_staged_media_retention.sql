ALTER TABLE property_media
  ADD COLUMN staged_at TIMESTAMP NULL DEFAULT NULL,
  ADD COLUMN staged_discarded BOOLEAN NOT NULL DEFAULT FALSE,
  ADD INDEX idx_property_media_staged_at (staged_at);

UPDATE property_media SET staged_at=CURRENT_TIMESTAMP WHERE edit_batch<>'';
