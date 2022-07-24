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

type PM_ROUTE struct {
	sq.TableStruct  `ddl:"unique=site_id,path"`
	SITE_ID         sq.UUIDField
	ROUTE_ID        sq.UUIDField `ddl:"primarykey"`
	PARENT_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	BASENAME        sq.StringField
	HANDLER         sq.StringField
	PATH            sq.StringField
}

type PM_ROUTE_HIERARCHY struct {
	sq.TableStruct    `ddl:"primarykey=route_id,ancestor_route_id"`
	ROUTE_ID          sq.UUIDField `ddl:"references=pm_route.route_id"`
	ANCESTOR_ROUTE_ID sq.UUIDField `ddl:"references={pm_route.route_id index}"`
	DEPTH             sq.NumberField
}
