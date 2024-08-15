CREATE TABLE dialog (
sender_id VARCHAR(255) NOT NULL,
getter_id VARCHAR(255) NOT NULL,
message text NOT NULL,
message_dt timestamp NOT NULL
);

SELECT create_distributed_table('dialog', 'message_dt');

-- insert into dialog(sender_id, getter_id, message, message_dt)
-- select
-- uuid_generate_v4()::TEXT,
-- uuid_generate_v4()::TEXT,
-- md5(random()::text),
-- timestamp '2014-01-10 20:00:00' +
--        random() * (timestamp '2014-01-20 20:00:00' -
--                    timestamp '2014-01-10 10:00:00')
-- from generate_series(1, 1000000) as i;