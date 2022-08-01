package pagemanager

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
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
	Mode string // "" | "offline" | "singlesite" | "multisite"
	FS   fs.ReadDirFS
}

type Pagemanager struct {
	mode string
	fs   fs.ReadDirFS
	wfs  WriteableFS
}

func NewPagemanager(c *Config) (*Pagemanager, error) {
	pm := &Pagemanager{
		mode: c.Mode,
		fs:   c.FS,
	}
	pm.wfs, _ = c.FS.(WriteableFS)
	return pm, nil
}

// Should this return an error? What errors should I bubble up?
func (pm *Pagemanager) Handler(name string, data map[string]any) (http.Handler, error) {
	if data == nil {
		data = make(map[string]any)
	}
	// What possible errors could exist?
	// .URL.Get
	// .QueryParams.Get
	// .Header.Get
	return nil, nil
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
