package service

const (
	sqlSelectSubs       = `select url from hooks where hooks.hook = $1::name;`
	sqlCreateHookTables = `
create table if not exists hooks
(
    name name not null
        constraint hooks_pk
            primary key
);

create table if not exists hook_subs
(
    hook name
        constraint hook_subs_hooks_name_fk
            references hooks
            on update cascade on delete cascade,
    url  text not null,
    id   uuid not null
);

create unique index if not exists hook_subs_hook_url_uindex
    on hook_subs (hook, url);

create unique index if not exists hook_subs_id_uindex
    on hook_subs (id);`
)