-- One database per service (database ownership principle).
-- Executed by the postgres entrypoint on first boot of an empty data volume.

CREATE DATABASE auth_db;
CREATE DATABASE user_db;
CREATE DATABASE post_db;
CREATE DATABASE notification_db;