package pagemanager

import (
	"database/sql"
	"io/fs"
)

type Pagemanager struct {
	mode    string // "offline", "singlesite", "multisite"
	siteID  [16]byte
	dialect string
	db      *sql.DB
}

type PageRenderer struct {
	fsys     fs.FS
	filename string
	config   []byte
}
