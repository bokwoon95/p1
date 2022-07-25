package pagemanager

import "github.com/bokwoon95/sq"

type PM_SITE struct {
	sq.TableStruct `ddl:"unique=domain,subdomain,tilde_prefix"`
	SITE_ID        sq.UUIDField `ddl:"primarykey"`
	DOMAIN         sq.StringField
	SUBDOMAIN      sq.StringField
	TILDE_PREFIX   sq.StringField
	DATA           sq.JSONField
	IS_PRIMARY     sq.BooleanField `ddl:"unique"`
}

// TODO: it's decided, page rendering will be baked into pagemanger and will
// not be implemented as a handler that is registered on startup.

type PM_ROUTE struct {
	sq.TableStruct  `ddl:"unique=site_id,path"`
	SITE_ID         sq.UUIDField `ddl:"notnull references={pm_site index}"`
	ROUTE_ID        sq.UUIDField `ddl:"primarykey"`
	PARENT_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	BASENAME        sq.StringField
	PATH            sq.StringField `ddl:"index"`
	TEMPLATE        sq.StringField
	DATA            sq.JSONField
	HANDLER         sq.StringField `ddl:"index"`
}

type PM_ROUTE_CLOSURE struct {
	sq.TableStruct    `ddl:"primarykey=route_id,ancestor_route_id"`
	ROUTE_ID          sq.UUIDField `ddl:"references=pm_route.route_id"`
	ANCESTOR_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	DEPTH             sq.NumberField
}
