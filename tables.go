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

// TODO: combine plugin+tag and plugin+role together so that I don't have to
// keep lugging around a reference to plugin everytime I need to reference a
// tag or role.

// TODO: what exactly does a handler receive?
// Handler receives the current user and his permissions on the current route
// (all automatically calculated through the combination of
// user+roles+route+tags). All the handler has to check is if the permission it
// is interested in is present in the permission list.

type PM_ROUTE struct {
	sq.TableStruct  `ddl:"unique=site_id,path"`
	SITE_ID         sq.UUIDField `ddl:"notnull references={pm_site index}"`
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

type PM_ROLE struct {
	sq.TableStruct `ddl:"primarykey=site_id,plugin,role"`
	SITE_ID        sq.UUIDField
	PLUGIN         sq.StringField
	ROLE           sq.StringField
}

type PM_TAG struct {
	sq.TableStruct `ddl:"primarykey=site_id,plugin,tag"`
	SITE_ID        sq.UUIDField
	PLUGIN         sq.StringField
	TAG            sq.StringField
}

type PM_ROLE_PERMISSION struct {
	sq.TableStruct `ddl:"primarykey=site_id,plugin,role,permission"`
	SITE_ID        sq.UUIDField
	PLUGIN         sq.StringField
	ROLE           sq.StringField
	PERMISSION     sq.StringField
}

type PM_TAG_PERMISSION struct {
	sq.TableStruct `ddl:"primarykey=site_id,plugin,tag,permission"`
	SITE_ID        sq.UUIDField
	PLUGIN         sq.StringField
	TAG            sq.StringField
	PERMISSION     sq.StringField
}

type PM_TAG_OWNER struct {
	sq.TableStruct `ddl:"primarykey=site_id,plugin,tag,role"`
	SITE_ID        sq.UUIDField
	PLUGIN         sq.StringField
	TAG            sq.StringField
	ROLE           sq.StringField
}

type PM_USER struct {
	sq.TableStruct         `ddl:"unique=site_id,username unique=site_id,email"`
	SITE_ID                sq.UUIDField `ddl:"notnull references={pm_site index}"`
	USER_ID                sq.UUIDField `ddl:"primarykey"`
	NAME                   sq.StringField
	USERNAME               sq.StringField `ddl:"notnull"`
	EMAIL                  sq.StringField
	PASSWORD_HASH          sq.StringField
	RESET_PASSWORD_TOKEN   sq.StringField
	RESET_PASSWORD_SENT_AT sq.TimeField
}

type PM_SESSION struct {
	sq.TableStruct
	SESSION_HASH sq.BinaryField
	USER_ID      sq.UUIDField `ddl:"references=pm_user"`
}

type PM_USER_ROLE struct {
	sq.TableStruct `ddl:"primarykey=user_id,plugin,role"`
	SITE_ID        sq.UUIDField `ddl:"notnull references={pm_site index}"`
	USER_ID        sq.UUIDField
	PLUGIN         sq.StringField
	ROLE           sq.StringField
	_              struct{} `ddl:"foreignkey={site_id,plugin,role reference=pm_role}"`
}

type PM_ROUTE_ROLE_PERMISSION struct {
	sq.TableStruct `ddl:"primarykey=route_id,plugin,role,permission"`
	SITE_ID        sq.UUIDField `ddl:"notnull references={pm_site index}"`
	ROUTE_ID       sq.UUIDField `ddl:"notnull references=pm_route"`
	PLUGIN         sq.StringField
	ROLE           sq.StringField
	PERMISSION     sq.StringField
}

type PM_ROUTE_TAG struct {
	sq.TableStruct `ddl:"primarykey=route_id,plugin,tag"`
	SITE_ID        sq.UUIDField `ddl:"notnull references={pm_site index}"`
	ROUTE_ID       sq.UUIDField `ddl:"notnull references=pm_route"`
	PLUGIN         sq.StringField
	TAG            sq.StringField
}
