SET NAMES utf8mb4;

INSERT INTO users (id, open_id, nickname, avatar_url, phone, status) VALUES
  ('qa-user-a', 'test:qa-user-a', '测试用户 A', '', '13800000001', 'active'),
  ('qa-user-b', 'test:qa-user-b', '测试用户 B', '', '13800000002', 'active'),
  ('qa-admin', 'test:qa-admin', '测试管理员', '', '13800000003', 'active'),
  ('qa-disabled', 'test:qa-disabled', '禁用测试用户', '', '13800000004', 'blocked')
ON DUPLICATE KEY UPDATE nickname=VALUES(nickname), phone=VALUES(phone), status=VALUES(status);

INSERT INTO admin_users (user_id, role, status, created_by)
VALUES ('qa-admin', 'admin', 'active', 'test-fixture')
ON DUPLICATE KEY UPDATE role='admin', status='active', created_by='test-fixture';

INSERT INTO properties (id,title,community,location,layout,rent_monthly,fee_text,description,status,owner_id)
VALUES ('qa-property-1','TEST Listing A','TEST Community A','TEST Address','1bd 1ba',1800,'TEST - no broker fee','Synthetic test fixture only','available','qa-user-a')
ON DUPLICATE KEY UPDATE title=VALUES(title), community=VALUES(community), location=VALUES(location), layout=VALUES(layout), rent_monthly=VALUES(rent_monthly), fee_text=VALUES(fee_text), description=VALUES(description), status='available', owner_id='qa-user-a';
