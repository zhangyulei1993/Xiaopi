ALTER TABLE rental_requests
  ADD COLUMN property_title VARCHAR(200) NOT NULL DEFAULT '' AFTER property_id;

UPDATE rental_requests r
LEFT JOIN properties p ON p.id = r.property_id
SET r.property_title = COALESCE(NULLIF(p.community, ''), NULLIF(p.title, ''), r.property_title)
WHERE r.request_type = 'appointment' AND r.property_title = '';
