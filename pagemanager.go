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

type Pagemanager struct {
	Mode string // "" | "offline" | "singlesite" | "multisite"
	FS   fs.ReadDirFS
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
