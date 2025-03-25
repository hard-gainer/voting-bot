box.cfg({listen = "0.0.0.0:3301"})

box.schema.user.create('storage', {password = 'password', if_not_exists = true})
box.schema.user.grant('storage', 'super', nil, nil, {if_not_exists = true})

if not box.space.polls then
    local polls = box.schema.space.create('polls', {
        if_not_exists = true,
        format = {
            {name = 'id', type = 'string'},
            {name = 'title', type = 'string'},
            {name = 'options', type = 'array'},
            {name = 'created_by', type = 'string'},
            {name = 'created_at', type = 'unsigned'},
            {name = 'is_active', type = 'boolean', default = true},
            {name = 'votes', type = 'map'} -- map[user_id] = option_index
        }
    })

    polls:create_index('primary', {
        if_not_exists = true,
        type = 'HASH',
        parts = {'id'}
    })

    polls:create_index('created_by', {
        if_not_exists = true,
        type = 'TREE',
        parts = {'created_by'}
    })

    polls:create_index('is_active', {
        if_not_exists = true,
        type = 'TREE',
        parts = {'is_active'}
    })
end

require('msgpack').cfg{encode_invalid_as_nil = true}