ALTER TABLE temporary_access_codes
  ADD COLUMN access_code CHAR(8) NULL AFTER code_hash;
