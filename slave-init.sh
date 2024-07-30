#!/bin/bash
set -e
# cat "$PGDATA"/postmaster.opts
# cat "$PGDATA"/postmaster.pid
# rm -rf "$PGDATA"/*

ipcs -m | grep postgres | awk '{print $2}' | xargs -I {} ipcrm -m {}

until pg_isready -h master -p 5432 -U postgres; do
 echo "Waiting for master to be ready..."
 sleep 2
done


PGPASSWORD=$POSTGRES_PASSWORD pg_basebackup -h master -D "/tmp/data_bkp/" -U replicator -vP -R --wal-method=stream

cp "$PGDATA"/postmaster.pid /tmp/data_bkp/
cp "$PGDATA"/postmaster.opts /tmp/data_bkp/

rm -rf "$PGDATA"/*

cp -r /tmp/data_bkp/ "$PGDATA"

rm -f "/var/run/postgresql/.s.PGSQL.5432.lock"

# ls -la "$PGDATA"

pg_ctl restart

# pg_ctl reload
# pg_ctl restart

# exec /docker-entrypoint-initdb.d/slave-init.sh /usr/lib/postgresql/16/bin/postgres "-D" "/var/lib/postgresql/data" "-c" "synchronous_standby_names=*" "-c" "listen_addresses=" " -p " "5432"
# cat "$PGDATA"/postmaster.pid
# top