#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
 CREATE ROLE replicator WITH REPLICATION PASSWORD 'pass' LOGIN;
 ALTER SYSTEM SET wal_level = replica;
 ALTER SYSTEM SET max_wal_senders = 10;
 ALTER SYSTEM SET wal_keep_size = '16MB';
 ALTER SYSTEM SET synchronous_commit = remote_apply;
 ALTER SYSTEM SET synchronous_standby_names = '*';
EOSQL

# Allow replication connections
echo "host replication all 0.0.0.0/0 md5" >> "$PGDATA/pg_hba.conf"

pg_ctl -D "$PGDATA" -m fast -w restart