package pagemanager

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
)

var bufpool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

type WriteableFS interface {
	Open(name string) (fs.File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	MkdirAll(name string, perm fs.FileMode) error
	RemoveAll(name string) error
}

type Config struct {
	Mode     string // "" | "offline" | "singlesite" | "multisite"
	FS       fs.ReadDirFS
	Handlers map[string]http.Handler
}

type Pagemanager struct {
	mode     string
	fs       fs.ReadDirFS
	wfs      WriteableFS
	handlers map[string]http.Handler
}

func NewPagemanager(c *Config) (*Pagemanager, error) {
	pm := &Pagemanager{
		mode:     c.Mode,
		fs:       c.FS,
		handlers: c.Handlers,
	}
	pm.wfs, _ = c.FS.(WriteableFS)
	return pm, nil
}

func (pm *Pagemanager) Handler(name string, data map[string]any) (http.Handler, error) {
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	var err error
	var file fs.File
	filenames := []string{"index.html", "index.md", "handler.txt"}
	for _, filename := range filenames {
		file, err = pm.fs.Open(path.Join(name, filename))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		break
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileinfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	filename := fileinfo.Name()
	switch filename {
	case "index.html":
	case "index.md":
	case "handler.txt":
	default:
		return nil, fmt.Errorf("invalid filename %q", filename)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data == nil {
			data = make(map[string]any)
		}
		data["URL"] = r.URL
		data["Header"] = r.Header
		data["QueryParams"], _ = url.ParseQuery(r.URL.RawQuery)
	}), nil
}

func (pm *Pagemanager) NotFound() http.Handler {
	// fallback to default 404.html if error is encountered.
	return nil
}

func (pm *Pagemanager) InternalServerError(err error) http.Handler {
	// fallback to default 500.html if error is encountered.
	return nil
}

func (pm *Pagemanager) Pagemanager(next http.Handler) http.Handler {
	pm.wfs, _ = pm.fs.(WriteableFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// domain, subdomain
		var domain, subdomain string
		if r.Host != "localhost" && r.Host != "127.0.0.1" && !strings.HasPrefix(r.Host, "localhost:") && !strings.HasPrefix(r.Host, "127.0.0.1:") {
			if i := strings.LastIndex(r.Host, "."); i >= 0 {
				domain = r.Host
				if j := strings.LastIndex(r.Host[:i], "."); j >= 0 {
					subdomain, domain = r.Host[:j], r.Host[j+1:]
				}
			}
		}
		var tildePrefix string
		urlPath := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(urlPath, "~") {
			if i := strings.Index(urlPath, "/"); i >= 0 {
				tildePrefix, urlPath = urlPath[:i], urlPath[i+1:]
			}
		}
		routeName := path.Join(domain, subdomain, tildePrefix, "pm-route", urlPath)
		handler, err := pm.Handler(routeName, nil)
		if errors.Is(err, fs.ErrNotExist) {
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			pm.InternalServerError(err).ServeHTTP(w, r)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
