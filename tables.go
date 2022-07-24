package pagemanager

import "github.com/bokwoon95/sq"

type PM_SITE struct {
	sq.TableStruct `ddl:"unique=domain,subdomain,tilde_prefix"`
	SITE_ID        sq.UUIDField `ddl:"primarykey"`
	DOMAIN         sq.StringField
	SUBDOMAIN      sq.StringField
	TILDE_PREFIX   sq.StringField
	IS_PRIMARY     sq.BooleanField `ddl:"unique"`
}

// plugins and handlers are registered by plugins on startup. The server itself
// will scan through the pm_route table and make sure that all mentioned
// plugins and handlers exist and have been registered. No need for foreign
// keys.

type PM_ROUTE struct {
	sq.TableStruct  `ddl:"unique=site_id,path"`
	SITE_ID         sq.UUIDField
	ROUTE_ID        sq.UUIDField `ddl:"primarykey"`
	PARENT_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	BASENAME        sq.StringField
	PATH            sq.StringField `ddl:"index"`
	PLUGIN          sq.StringField
	HANDLER         sq.StringField
	CONFIG          sq.JSONField
	_               struct{} `ddl:"index=plugin,handler"`
}

type PM_ROUTE_HIERARCHY struct {
	sq.TableStruct    `ddl:"primarykey=route_id,ancestor_route_id"`
	ROUTE_ID          sq.UUIDField `ddl:"references=pm_route.route_id"`
	ANCESTOR_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	DEPTH             sq.NumberField
}
