#!/bin/bash
set -e

rm -rf "$PGDATA"/*

ipcs -m | grep postgres | awk '{print $2}' | xargs -I {} ipcrm -m {}

until pg_isready -h master -p 5432 -U postgres; do
 echo "Waiting for master to be ready..."
 sleep 2
done

PGPASSWORD=$POSTGRES_PASSWORD pg_basebackup -h master -D "$PGDATA" -U replicator -vP -R

rm -f "/var/run/postgresql/.s.PGSQL.5432.lock"

exec docker-entrypoint.sh postgres