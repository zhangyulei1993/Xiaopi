ALTER TABLE subscription_deliveries
  ADD COLUMN result VARCHAR(16) NOT NULL DEFAULT 'uncertain',
  ADD COLUMN last_error VARCHAR(500) NOT NULL DEFAULT '';
