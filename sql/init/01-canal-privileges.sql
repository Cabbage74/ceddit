-- Grant replication privileges for Canal (which connects from the canal container).
-- The root user should already have these on localhost, but Canal connects
-- from a different host, so we need to grant from '%'.
GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'root'@'%';
FLUSH PRIVILEGES;
