package pagemanager

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
	"sync"
	"text/template"
)

var bufpool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

type Pagemanager struct {
	Mode     string // "" | "offline" | "singlesite" | "multisite"
	FS       FS
	Router   Router
	Handlers map[string]http.Handler
	Dialect  string
	DB       *sql.DB
}

type FS interface {
	Open(name string) (fs.File, error)
	Stat(name string) (fs.FileInfo, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(name string, perm fs.FileMode) error
	RemoveAll(name string) error
}

type Router interface {
	Route(name string) (fs.File, error)
}

func (pm *Pagemanager) Pagemanager(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain, subdomain, tildePrefix, pathName := splitURL(r.Host, r.URL.Path)
		// Check pm-site.
		name := path.Join("pm-site", domain, subdomain, tildePrefix, pathName, "index.html")
		if file, err := pm.FS.Open(name); err == nil {
			defer file.Close()
			_, err = io.Copy(w, file)
			if err != nil {
				log.Println(err)
			}
			return
		}
		// Check router.
		name = path.Join(domain, subdomain, tildePrefix, pathName)
		file, err := pm.Router.Route(name)
		if errors.Is(err, fs.ErrNotExist) {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			log.Println(err)
			return
		}
		fileinfo, err := file.Stat()
		if err != nil {
			log.Println(err)
			return
		}
		filename := fileinfo.Name()
		buf := bufpool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufpool.Put(buf)
		switch filename {
		case "index.html":
			_, err = buf.ReadFrom(file)
			if err != nil {
				log.Println(err)
				return
			}
			// TODO: what funcs?
			// {{ $posts := get "github.com/bokwoon95/pagemanager/pm-blog.Posts" }}
			template.New(name)
		case "index.md":
		case "handler.txt":
		default:
		}
		next.ServeHTTP(w, r)
	})
}

func splitURL(host, requestPath string) (domain, subdomain, tildePrefix, pathName string) {
	pathName = requestPath
	if len(requestPath) > 2 && requestPath[1] == '~' {
		if i := strings.Index(requestPath[1:], "/"); i > 0 {
			tildePrefix, pathName = requestPath[1:i], requestPath[i:]
		}
	}
	if host == "localhost" || strings.HasPrefix(host, "localhost:") {
		return "", "", tildePrefix, pathName
	}
	domain = host
	if i := strings.LastIndex(host, "."); i > 0 {
		if j := strings.LastIndex(host[:i], "."); j > 0 {
			if k := strings.LastIndex(host[:j], "."); k > 0 {
				subdomain, domain = host[:k], host[k:]
			}
		}
	}
	return domain, subdomain, tildePrefix, pathName
}

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

// The only 3 dependencies the pagemanager will inject each handler with: fsys fs.FS, db *sql.DB, config []byte.

// Won't they want to know the SiteID, RouteID and UserID as well?

// When registering plugins, each plugin must provide an fs.FS containing the JSON schema files for each handler followed by a map[string]func(fsys fs.FS, db *sql.DB, config []byte) http.Handler
