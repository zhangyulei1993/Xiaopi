DELETE FROM property_favorites WHERE user_id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM user_footprints WHERE user_id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM rental_requests WHERE user_id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM property_contact_events WHERE user_id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM operation_logs WHERE operator_id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM temporary_access_codes WHERE phone IN ('13800000001','13800000002','13800000003','13800000004');
DELETE FROM admin_users WHERE user_id='qa-admin';
DELETE FROM users WHERE id IN ('qa-user-a','qa-user-b','qa-admin','qa-disabled');
DELETE FROM properties WHERE id='qa-property-1';
