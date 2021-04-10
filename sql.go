package service

// subscriptions query
const (
	sqlSubscribe            = `insert into web_hooks.subscribers (hook_name, url, pass_code) values ($1::name, $2::text, $3::uuid);`
	sqlUnsubscribe          = `delete from web_hooks.subscribers where hook_name = $1::name and url = $2::text and pass_code = $3::uuid;`
	sqlSelectSubCode        = `select pass_code from web_hooks.subscribers where hook_name = $1::name and url = $2::text`
	sqlSelectSubs           = `select url, pass_code, err_count from web_hooks.subscribers where hook_name = $1::name;`
	sqlResetSubErrCount     = `update web_hooks.subscribers set err_count = 0 where hook_name = $1::name and url = $2::text;`
	sqlIncrementSubErrCount = `update web_hooks.subscribers set err_count = err_count+1 where hook_name = $1::name and url = $2::text;`
	sqlDeleteSub            = `delete from web_hooks.subscribers where hook_name = $1::name and url = $2::text;`
)

// hooks query
const (
	sqlSelectHooks = `select * from web_hooks.hooks;`
	sqlAddHook     = `insert into web_hooks.hooks (name, function_name) values ($1::name, $2::name) on conflict (name) do nothing;`
	sqlDeleteHook  = `delete from web_hooks.hooks where name = $1::name;`
)

// Структура таблицы hooks в БД
type tHook struct {
	Name     string
	Function string
}

const createHookSchema = `
create schema if not exists web_hooks;

create table if not exists web_hooks.hooks
(
    Name Name not null
        constraint hooks_pk
            primary key,
	function_name name
);

create table if not exists web_hooks.subscribers
(
    hook_name Name
        constraint subscribers_hooks_name_fk
            references web_hooks.hooks
            on update cascade on delete cascade,
    url       text              not null,
    pass_code uuid              not null,
    err_count integer default 0 not null
);

create unique index if not exists subscribers_hook_name_url_uindex
    on web_hooks.subscribers (hook_name, url);`
