package pagemanager

import (
	"bytes"
	"io/fs"
	"net/http"
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

func (pm *Pagemanager) Handler(name string, data map[string]any) http.Handler {
	return nil
}

func (pm *Pagemanager) NotFound(data map[string]any) http.Handler {
	return nil
}

func (pm *Pagemanager) InternalServerError(data map[string]any) http.Handler {
	return nil
}

func (pm *Pagemanager) Pagemanager(next http.Handler) http.Handler {
	pm.wfs, _ = pm.fs.(WriteableFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
