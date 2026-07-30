#!/bin/bash
# iCloud Mailflow — demo data generator
# Usage: bash scripts/demo.sh && go run ./cmd/mailflow/ -data=./demo

DEMO=./demo
mkdir -p $DEMO

DB=$DEMO/mailflow.db
rm -f $DB

sqlite3 $DB <<'SQL'
-- Settings
INSERT INTO settings VALUES ('admin_password_hash','$2a$10$KxP1ZxyoB3kFpN8jVq3R5uRr7mW4tL2sH6yA1bB8cD0eF','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('imap_email','user@icloud.com','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('imap_password','app-password-encrypted','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('source_folder','Processing','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('poll_interval','300','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('poll_batch','50','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('log_keep','1000','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('timezone','Europe/Copenhagen','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('font_mono','true','2026-07-30 10:00:00');
INSERT INTO settings VALUES ('polling_enabled','true','2026-07-30 10:00:00');

-- Sessions
INSERT INTO sessions VALUES ('demo-session-token-abc123','2099-12-31 23:59:59');

-- Folders
INSERT INTO folders VALUES (1,'INBOX','INBOX','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (2,'Archive','Archive','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (3,'Sent','Sent','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (4,'Trash','Trash','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (5,'Work','Work','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (6,'Family','Family','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (7,'Bills','Bills','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (8,'Shopping','Shopping','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (9,'Gaming','Gaming','','2026-07-30 10:00:00');
INSERT INTO folders VALUES (10,'Newsletters','Newsletters','','2026-07-30 10:00:00');

-- Rules
INSERT INTO rules VALUES (1,'Work','Move work emails to Work folder',0,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (2,'Newsletters','Archive marketing emails and mark as read',1,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (3,'Family','Move family emails to Family folder',2,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (4,'Shopping','Archive shopping receipts',3,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (5,'Gaming','Flag and move gaming emails',4,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (6,'Bills','Move invoices to Bills folder',5,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');
INSERT INTO rules VALUES (7,'_catch_all','Built-in catch-all — moves unmatched mail to INBOX',999,1,'2026-07-30 10:00:00','2026-07-30 10:00:00');

-- Condition groups
INSERT INTO condition_groups VALUES (1,1,NULL,'OR');
INSERT INTO condition_groups VALUES (2,2,NULL,'OR');
INSERT INTO condition_groups VALUES (3,3,NULL,'AND');
INSERT INTO condition_groups VALUES (4,4,NULL,'OR');
INSERT INTO condition_groups VALUES (5,5,NULL,'AND');
INSERT INTO condition_groups VALUES (6,6,NULL,'AND');

-- Conditions
-- Work: from @company.com OR cc @work.com
INSERT INTO conditions VALUES (1,1,'from','contains','@company.com');
INSERT INTO conditions VALUES (2,1,'cc','contains','@work.com');

-- Newsletters: from @marketing.co OR subject contains "newsletter"
INSERT INTO conditions VALUES (3,2,'from','contains','@marketing.co');
INSERT INTO conditions VALUES (4,2,'subject','contains','weekly');

-- Family: from contains @family.com AND to contains user@icloud.com
INSERT INTO conditions VALUES (5,3,'from','contains','@family.com');
INSERT INTO conditions VALUES (6,3,'to','contains','user@icloud.com');

-- Shopping: from @amazon.com OR from @shopify.com OR from @etsy.com
INSERT INTO conditions VALUES (7,4,'from','contains','@amazon.com');
INSERT INTO conditions VALUES (8,4,'from','contains','@shopify.com');
INSERT INTO conditions VALUES (9,4,'from','contains','@etsy.com');

-- Gaming: from @discord.com AND subject contains "mentioned"
INSERT INTO conditions VALUES (10,5,'from','contains','@discord.com');
INSERT INTO conditions VALUES (11,5,'subject','contains','mentioned');

-- Bills: subject contains "invoice" AND subject contains "due"
INSERT INTO conditions VALUES (12,6,'subject','contains','invoice');
INSERT INTO conditions VALUES (13,6,'subject','contains','due');

-- Actions
INSERT INTO actions VALUES (1,1,'move_to_folder','Work');
INSERT INTO actions VALUES (2,2,'move_to_folder','Newsletters');
INSERT INTO actions VALUES (3,2,'mark_as_read','');
INSERT INTO actions VALUES (4,3,'move_to_folder','Family');
INSERT INTO actions VALUES (5,4,'move_to_folder','Shopping');
INSERT INTO actions VALUES (6,4,'mark_as_read','');
INSERT INTO actions VALUES (7,5,'mark_as_read','');
INSERT INTO actions VALUES (8,5,'set_flag','\Flagged');
INSERT INTO actions VALUES (9,5,'move_to_folder','Gaming');
INSERT INTO actions VALUES (10,6,'move_to_folder','Bills');
INSERT INTO actions VALUES (11,7,'move_to_folder','INBOX');

-- Contacts
INSERT INTO contacts VALUES ('alice@company.com','Alice Johnson','2026-07-20 08:00:00','2026-07-30 09:00:00',12);
INSERT INTO contacts VALUES ('bob@work.com','Bob Smith','2026-07-21 09:00:00','2026-07-30 10:00:00',8);
INSERT INTO contacts VALUES ('carol@marketing.co','Carol Davis','2026-07-22 10:00:00','2026-07-30 08:00:00',15);
INSERT INTO contacts VALUES ('dave@family.com','Dave Wilson','2026-07-23 11:00:00','2026-07-29 14:00:00',7);
INSERT INTO contacts VALUES ('eve@company.com','Eve Brown','2026-07-24 12:00:00','2026-07-30 11:00:00',10);
INSERT INTO contacts VALUES ('frank@amazon.com','Amazon Orders','2026-07-25 13:00:00','2026-07-30 12:00:00',22);
INSERT INTO contacts VALUES ('grace@shopify.com','Shopify Notifications','2026-07-26 14:00:00','2026-07-30 13:00:00',18);
INSERT INTO contacts VALUES ('hank@etsy.com','Etsy Shop','2026-07-27 15:00:00','2026-07-30 14:00:00',9);
INSERT INTO contacts VALUES ('iris@discord.com','Discord Notifications','2026-07-28 16:00:00','2026-07-30 15:00:00',14);
INSERT INTO contacts VALUES ('jack@gmail.com','Jack Taylor','2026-07-29 17:00:00','2026-07-30 16:00:00',5);
INSERT INTO contacts VALUES ('kate@outlook.com','Kate Miller','2026-07-30 08:00:00','2026-07-30 17:00:00',3);
INSERT INTO contacts VALUES ('leo@proton.me','Leo Anderson','2026-07-25 18:00:00','2026-07-28 18:00:00',4);
INSERT INTO contacts VALUES ('mia@company.com','Mia Thomas','2026-07-26 19:00:00','2026-07-30 18:00:00',6);
INSERT INTO contacts VALUES ('nora@work.com','Nora Jackson','2026-07-27 20:00:00','2026-07-29 19:00:00',7);
INSERT INTO contacts VALUES ('owen@family.com','Owen White','2026-07-28 21:00:00','2026-07-30 19:00:00',4);

-- Activity log
INSERT INTO message_log VALUES (1,'2026-07-30 10:05:00',12,'Q4 Budget Review','alice@company.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (2,'2026-07-30 10:06:00',34,'Your Weekly Newsletter','carol@marketing.co','Newsletters','move_to_folder','Newsletters','success');
INSERT INTO message_log VALUES (3,'2026-07-30 10:06:00',34,'Your Weekly Newsletter','carol@marketing.co','Newsletters','mark_as_read','','success');
INSERT INTO message_log VALUES (4,'2026-07-30 10:07:00',56,'Family BBQ this weekend','dave@family.com','Family','move_to_folder','Family','success');
INSERT INTO message_log VALUES (5,'2026-07-30 10:08:00',78,'Your Amazon order has shipped','frank@amazon.com','Shopping','move_to_folder','Shopping','success');
INSERT INTO message_log VALUES (6,'2026-07-30 10:08:00',78,'Your Amazon order has shipped','frank@amazon.com','Shopping','mark_as_read','','success');
INSERT INTO message_log VALUES (7,'2026-07-30 10:09:00',89,'You were mentioned in #general','iris@discord.com','Gaming','mark_as_read','','success');
INSERT INTO message_log VALUES (8,'2026-07-30 10:09:00',89,'You were mentioned in #general','iris@discord.com','Gaming','set_flag','\Flagged','success');
INSERT INTO message_log VALUES (9,'2026-07-30 10:09:00',89,'You were mentioned in #general','iris@discord.com','Gaming','move_to_folder','Gaming','success');
INSERT INTO message_log VALUES (10,'2026-07-30 10:10:00',92,'Invoice #2026-0892 Due','bills@service.com','Bills','move_to_folder','Bills','success');
INSERT INTO message_log VALUES (11,'2026-07-30 10:11:00',105,'Meeting notes from yesterday','mia@company.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (12,'2026-07-30 10:12:00',118,'50% OFF - Today Only!','deals@shopify.com','Newsletters','move_to_folder','Newsletters','success');
INSERT INTO message_log VALUES (13,'2026-07-30 10:12:00',118,'50% OFF - Today Only!','deals@shopify.com','Newsletters','mark_as_read','','success');

INSERT INTO message_log VALUES (14,'2026-07-29 09:15:00',201,'Project Alpha Update','nora@work.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (15,'2026-07-29 09:16:00',223,'New listing: Handmade mug','hank@etsy.com','Shopping','move_to_folder','Shopping','success');
INSERT INTO message_log VALUES (16,'2026-07-29 09:16:00',223,'New listing: Handmade mug','hank@etsy.com','Shopping','mark_as_read','','success');
INSERT INTO message_log VALUES (17,'2026-07-29 09:17:00',234,'Mom''s birthday present','dave@family.com','Family','move_to_folder','Family','success');
INSERT INTO message_log VALUES (18,'2026-07-29 09:18:00',256,'Payment Confirmation #456','receipt@shopify.com','_catch_all','move_to_folder','INBOX','success');

INSERT INTO message_log VALUES (19,'2026-07-28 14:22:00',301,'Q3 Sales Report','alice@company.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (20,'2026-07-28 14:23:00',312,'Flash Sale - 2 Hours Only!','sale@amazon.com','Shopping','move_to_folder','Shopping','success');
INSERT INTO message_log VALUES (21,'2026-07-28 14:23:00',312,'Flash Sale - 2 Hours Only!','sale@amazon.com','Shopping','mark_as_read','','success');
INSERT INTO message_log VALUES (22,'2026-07-28 14:24:00',325,'Server downtime tonight','iris@discord.com','Gaming','mark_as_read','','success');
INSERT INTO message_log VALUES (23,'2026-07-28 14:24:00',325,'Server downtime tonight','iris@discord.com','Gaming','set_flag','\Flagged','success');

INSERT INTO message_log VALUES (24,'2026-07-27 11:30:00',401,'Team lunch tomorrow','bob@work.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (25,'2026-07-27 11:31:00',415,'Invoice Overdue #IMP-001','billing@corp.com','Bills','move_to_folder','Bills','success');
INSERT INTO message_log VALUES (26,'2026-07-27 11:32:00',428,'July Newsletter','carol@marketing.co','Newsletters','move_to_folder','Newsletters','success');
INSERT INTO message_log VALUES (27,'2026-07-27 11:32:00',428,'July Newsletter','carol@marketing.co','Newsletters','mark_as_read','','success');

INSERT INTO message_log VALUES (28,'2026-07-26 16:45:00',501,'Vacation photos','owen@family.com','Family','move_to_folder','Family','success');
INSERT INTO message_log VALUES (29,'2026-07-26 16:46:00',515,'Holiday sale','kate@outlook.com','_catch_all','move_to_folder','INBOX','success');

INSERT INTO message_log VALUES (30,'2026-07-25 08:00:00',601,'Weekly Standup Notes','mia@company.com','Work','move_to_folder','Work','success');
INSERT INTO message_log VALUES (31,'2026-07-25 08:01:00',610,'Your Etsy order has shipped','hank@etsy.com','Shopping','move_to_folder','Shopping','success');
INSERT INTO message_log VALUES (32,'2026-07-25 08:01:00',610,'Your Etsy order has shipped','hank@etsy.com','Shopping','mark_as_read','','success');
INSERT INTO message_log VALUES (33,'2026-07-24 14:15:00',715,'Invoice Paid #PAID-882','receipts@shopify.com','Shopping','move_to_folder','Shopping','success');
INSERT INTO message_log VALUES (34,'2026-07-24 14:16:00',720,'Re: Birthday dinner','dave@family.com','Family','move_to_folder','Family','success');
INSERT INTO message_log VALUES (35,'2026-07-23 10:30:00',801,'Domain renewal notice','noreply@registrar.com','_catch_all','move_to_folder','INBOX','success');

-- Config
INSERT INTO settings VALUES ('folders_synced','true','2026-07-30 10:00:00');
SQL

echo "Demo data created at $DEMO/"
echo ""
echo "Run: go run ./cmd/mailflow/ -data=./demo"
echo "Login with session token in cookie: mailflow_session=demo-session-token-abc123"
