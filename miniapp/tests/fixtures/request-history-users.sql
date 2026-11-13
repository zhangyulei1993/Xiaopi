INSERT INTO rental_requests
(id,user_id,request_type,property_id,property_title,contact_name,phone,preferred_time,location,note,status,follow_up_note,created_at)
VALUES
('qa-history-e2e-a','qa-user-a','appointment','qa-property-1','QA History Listing A','QA History Person A','13800000001','QA Monday 10:30','TEST Address','Synthetic history fixture','contacted','QA Followup A','2025-06-15 12:00:00'),
('qa-history-e2e-b','qa-user-b','appointment','qa-property-1','QA History Listing B','QA History Person B','13800000002','QA Tuesday 15:30','TEST Address','Synthetic history fixture','contacted','QA Followup B','2025-07-16 12:00:00')
ON DUPLICATE KEY UPDATE
property_title=VALUES(property_title), contact_name=VALUES(contact_name), preferred_time=VALUES(preferred_time),
status=VALUES(status), follow_up_note=VALUES(follow_up_note), created_at=VALUES(created_at);
