-- init.lua

-- Initialize Tarantool
box.cfg {
    listen = 3301
}

-- Create space and indexes if not exists
if not box.space.dialog then
    box.schema.space.create('dialog', { if_not_exists = true })
    box.space.dialog:format({
        {name = 'sender_id', type = 'string'},
        {name = 'getter_id', type = 'string'},
        {name = 'message', type = 'string'},
        {name = 'message_dt', type = 'number'}
    })
    box.space.dialog:create_index('primary', {parts = {1, 'string', 2, 'string', 4, 'number'}})
end

-- Define Lua functions (stored procedures)
function initialize_dialog()
    -- Ensure space and indexes are created
    if not box.space.dialog then
        box.schema.space.create('dialog', { if_not_exists = true })
        box.space.dialog:format({
            {name = 'sender_id', type = 'string'},
            {name = 'getter_id', type = 'string'},
            {name = 'message', type = 'string'},
            {name = 'message_dt', type = 'number'}
        })
        box.space.dialog:create_index('primary', {parts = {1, 'string', 2, 'string', 4, 'number'}})
    end
end

function dialog_send(sender_id, getter_id, message)
    box.space.dialog:insert{sender_id, getter_id, message, os.time()}
end

function dialog_list(user_id, recipient_id)
    local result = {}
    for _, tuple in box.space.dialog.index.primary:pairs({user_id, recipient_id}, {iterator = 'EQ'}) do
        table.insert(result, {from = tuple.sender_id, to = tuple.getter_id, text = tuple.message})
    end
    return result
end