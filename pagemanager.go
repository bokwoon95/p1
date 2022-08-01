package pagemanager

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/template/parse"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
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
	FS       fs.FS
	Handlers map[string]http.Handler
}

type Pagemanager struct {
	mode     string
	fs       fs.FS
	wfs      WriteableFS
	handlers map[string]http.Handler
}

func New(c *Config) (*Pagemanager, error) {
	pm := &Pagemanager{
		mode:     c.Mode,
		fs:       c.FS,
		handlers: c.Handlers,
	}
	pm.wfs, _ = c.FS.(WriteableFS)
	return pm, nil
}

func (pm *Pagemanager) Template(name string, r io.Reader) (*template.Template, error) {
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	_, err := buf.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	body := buf.String()
	mainTemplate, err := template.New(name).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	if strings.HasSuffix(name, ".md") {
		md := goldmark.New(
			goldmark.WithParserOptions(
				parser.WithAttribute(),
			),
			goldmark.WithExtensions(
				extension.Table,
				highlighting.NewHighlighting(
					highlighting.WithStyle("dracula"),
				),
			),
			goldmark.WithRendererOptions(
				goldmarkhtml.WithUnsafe(),
			),
		)
		markdownTemplate := template.New("")
		for _, t := range mainTemplate.Templates() {
			buf.Reset()
			err = md.Convert([]byte(t.Tree.Root.String()), buf)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", name, t.Name(), err)
			}
			_, err = markdownTemplate.New(t.Name()).Parse(buf.String())
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", name, t.Name(), err)
			}
		}
		mainTemplate = markdownTemplate
	}

	var templateNames []string
	nodes := make([]parse.Node, 0, len(mainTemplate.Tree.Root.Nodes))
	for i := len(mainTemplate.Tree.Root.Nodes) - 1; i >= 0; i-- {
		nodes = append(nodes, mainTemplate.Tree.Root.Nodes[i])
	}
	var node parse.Node
	for len(nodes) > 0 {
		node, nodes = nodes[len(nodes)-1], nodes[:len(nodes)-1]
		switch node := node.(type) {
		case *parse.ListNode:
			for i := len(node.Nodes) - 1; i >= 0; i-- {
				nodes = append(nodes, node.Nodes[i])
			}
		case *parse.TemplateNode:
			templateNames = append(templateNames, node.Name)
		}
	}

	baseTemplate := template.New("")
	for _, templateName := range templateNames {
		if !strings.HasSuffix(templateName, ".html") {
			continue
		}
		file, err := pm.fs.Open(path.Join("pm-template", templateName))
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", name, templateName, err)
		}
		buf.Reset()
		_, err = buf.ReadFrom(file)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", name, templateName, err)
		}
		_ = file.Close()
		body := buf.String()
		_, err = baseTemplate.New(templateName).Parse(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", name, templateName, err)
		}
	}

	for _, t := range mainTemplate.Templates() {
		_, err = baseTemplate.AddParseTree(t.Name(), t.Tree)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", name, t.Name(), err)
		}
	}

	pageTemplate := baseTemplate.Lookup(name)
	return pageTemplate, nil
}

func (pm *Pagemanager) Error(w http.ResponseWriter, r *http.Request, msg string, code int) {
	statusCode := strconv.Itoa(code)
	msg = statusCode + " " + http.StatusText(code) + "\n" + msg
	domain, subdomain := splitHost(r.Host)
	tildePrefix, _ := splitPath(r.URL.Path)
	name := path.Join(domain, subdomain, tildePrefix, "pm-template", statusCode+".html")
	file, err := pm.fs.Open(name)
	if err != nil {
		http.Error(w, msg, code)
		return
	}
	defer file.Close()
	tmpl, err := pm.Template(name, file)
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	data := map[string]any{
		"URL":    r.URL,
		"Header": r.Header,
		"Msg":    msg,
	}
	data["QueryParams"], _ = url.ParseQuery(r.URL.RawQuery)
	err = tmpl.ExecuteTemplate(buf, name, data)
	if err != nil {
		http.Error(w, msg+"\n\n(error executing "+name+": "+err.Error()+")", code)
		return
	}
	_, _ = buf.WriteTo(w)
}

func (pm *Pagemanager) NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pm.Error(w, r, r.RequestURI, 404)
	})
}

func (pm *Pagemanager) InternalServerError(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pm.Error(w, r, err.Error(), 500)
	})
}

func (pm *Pagemanager) Handler(name string, data map[string]any) (http.Handler, error) {
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
	handlerPath := path.Join(name, filename)

	if filename == "handler.txt" {
		var b strings.Builder
		_, err = io.Copy(&b, file)
		if err != nil {
			return nil, err
		}
		handlerName := b.String()
		handler := pm.handlers[handlerName]
		if handler == nil {
			return nil, fmt.Errorf("%s: handler %q does not exist", handlerPath, handlerName)
		}
		return handler, nil
	}

	pageTemplate, err := pm.Template(handlerPath, file)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data == nil {
			data = make(map[string]any)
		}
		data["URL"] = r.URL
		data["Header"] = r.Header
		data["QueryParams"], _ = url.ParseQuery(r.URL.RawQuery)
		buf := bufpool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufpool.Put(buf)
		err = pageTemplate.ExecuteTemplate(buf, handlerPath, data)
		if err != nil {
			pm.InternalServerError(err).ServeHTTP(w, r)
			return
		}
		_, _ = buf.WriteTo(w)
	}), nil
}

func (pm *Pagemanager) Pagemanager(next http.Handler) http.Handler {
	pm.wfs, _ = pm.fs.(WriteableFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain, subdomain := splitHost(r.Host)
		tildePrefix, urlPath := splitPath(r.URL.Path)
		if strings.HasPrefix(urlPath, "pm-static") {
			// file, err := pm.fs.Open(urlPath)
			// if errors.Is(err, fs.ErrNotExist) {
			// 	if !strings.HasPrefix(urlPath, "pm-static/pm-template") {
			// 		pm.NotFound().ServeHTTP(w, r)
			// 		return
			// 	}
			// }
			// if err != nil && !errors.Is(err, fs.ErrNotExist) {
			// 	pm.InternalServerError(err).ServeHTTP(w, r)
			// 	return
			// }
			// return
		}
		name := path.Join(domain, subdomain, tildePrefix, "pm-route", urlPath)
		handler, err := pm.Handler(name, nil)
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

func splitHost(host string) (domain, subdomain string) {
	if host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "localhost:") || strings.HasPrefix(host, "127.0.0.1:") {
		return "", ""
	}
	if i := strings.LastIndex(host, "."); i >= 0 {
		domain = host
		if j := strings.LastIndex(host[:i], "."); j >= 0 {
			subdomain, domain = host[:j], host[j+1:]
		}
	}
	return domain, subdomain
}

func splitPath(path string) (tildePrefix, urlPath string) {
	// TODO: Langcode also needs to be extracted from the path. How to handle
	// this? Per-site langcode.txt?
	urlPath = strings.TrimPrefix(path, "/")
	if strings.HasPrefix(urlPath, "~") {
		if i := strings.Index(urlPath, "/"); i >= 0 {
			tildePrefix, urlPath = urlPath[:i], urlPath[i+1:]
		}
	}
	return tildePrefix, urlPath
}
