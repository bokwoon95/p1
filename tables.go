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

type PM_PLUGIN struct {
	sq.TableStruct
	PLUGIN        sq.StringField `ddl:"primarykey"`
	CONFIG        sq.JSONField
	CONFIG_SCHEMA sq.JSONField
}

type PM_HANDLER struct {
	sq.TableStruct `ddl:"primarykey=plugin,handler"`
	PLUGIN         sq.StringField `ddl:"references=pm_plugin"`
	HANDLER        sq.StringField
	CONFIG_SCHEMA  sq.JSONField
}

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
	_               struct{} `ddl:"foreignkey={plugin,handler references=pm_handler}"`
}

type PM_ROUTE_HIERARCHY struct {
	sq.TableStruct    `ddl:"primarykey=route_id,ancestor_route_id"`
	ROUTE_ID          sq.UUIDField `ddl:"references=pm_route.route_id"`
	ANCESTOR_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	DEPTH             sq.NumberField
}
