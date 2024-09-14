#!/bin/bash
set -e

# Configure PostgreSQL for replication

# Modify postgresql.conf to enable replication
echo "Configuring postgresql.conf for replication"
cat >> /var/lib/postgresql/data/postgresql.conf <<EOF
wal_level = replica
max_wal_senders = 3
wal_keep_size = 64
hot_standby = on
EOF

# Modify pg_hba.conf to allow replication connections from replicas
echo "Configuring pg_hba.conf for replication"
cat >> /var/lib/postgresql/data/pg_hba.conf <<EOF
# Allow replication connections from any IP address
host replication repl_user 0.0.0.0/0 md5
EOF

echo "PostgreSQL replication configuration complete"