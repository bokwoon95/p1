package pagemanager

import (
	"database/sql"
	"io/fs"
	"net/http"
)

type Pagemanager struct {
	mode    string // "offline", "singlesite", "multisite"
	siteID  [16]byte
	dialect string
	db      *sql.DB
}

// The only 3 dependencies the pagemanager will inject each handler with: fsys fs.FS, db *sql.DB, config []byte.

// Won't they want to know the SiteID, RouteID and UserID as well?

// When registering plugins, each plugin must provide an fs.FS containing the JSON schema files for each handler followed by a map[string]func(fsys fs.FS, db *sql.DB, config []byte) http.Handler

// The problem with bundling config and data together is that when the user switches to a different template the data form cannot change accordingly.

type PageRenderer struct {
	fsys     fs.FS
	filename string
	config   []byte
}

func (pr *PageRenderer) HandlerName() string {
	return ""
}

func (pr *PageRenderer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}
