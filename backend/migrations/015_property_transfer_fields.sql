ALTER TABLE properties
  ADD COLUMN listing_source VARCHAR(16) NOT NULL DEFAULT '直租' AFTER rent_type,
  ADD COLUMN remaining_lease_months INT NOT NULL DEFAULT 0 AFTER listing_source,
  ADD COLUMN transfer_deposit BIGINT NOT NULL DEFAULT 0 AFTER remaining_lease_months;
