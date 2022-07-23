package pagemanager

import "database/sql"

type Pagemanager struct {
	mode    string // "offline", "singlesite", "multisite"
	siteID  [16]byte
	dialect string
	db      *sql.DB
}
