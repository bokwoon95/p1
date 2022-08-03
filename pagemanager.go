package pagemanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template/parse"
	"time"
	"unicode"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/sync/errgroup"
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
	Queries  map[string]func(*url.URL, ...string) (any, error)
}

type Pagemanager struct {
	mode     string
	fs       fs.FS
	wfs      WriteableFS
	handlers map[string]http.Handler
	queries  map[string]func(*url.URL, ...string) (any, error)
}

func New(c *Config) (*Pagemanager, error) {
	pm := &Pagemanager{
		mode:     c.Mode,
		fs:       c.FS,
		handlers: c.Handlers,
		queries:  c.Queries,
	}
	if pm.queries == nil {
		pm.queries = make(map[string]func(*url.URL, ...string) (any, error))
	}
	funcs := Funcs{
		fs:      c.FS,
		queries: pm.queries,
	}
	pm.queries["github.com/pagemanager/pagemanager.Funcs.Index"] = funcs.Index
	pm.wfs, _ = c.FS.(WriteableFS)
	return pm, nil
}

var markdownConverter = goldmark.New(
	goldmark.WithParserOptions(
		parser.WithAttribute(),
	),
	goldmark.WithExtensions(
		extension.Table,
		highlighting.NewHighlighting(
			highlighting.WithStyle("dracula"), // TODO: eventually this will have to be user-configurable. Maybe even dynamically configurable from the front end (this will have to become a property on Pagemanager itself.
		),
	),
	goldmark.WithRendererOptions(
		goldmarkhtml.WithUnsafe(),
	),
)

func Markdownify(in *template.Template, funcmap map[string]any) (out *template.Template, err error) {
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	if funcmap != nil {
		out = template.New("").Funcs(funcmap)
	} else {
		out, err = in.Clone()
		if err != nil {
			return nil, err
		}
	}
	for _, t := range in.Templates() {
		if t.Tree == nil {
			continue
		}
		name := t.Name()
		isDataTemplate := len(name) > 0 && unicode.IsUpper(rune(name[0]))
		if !isDataTemplate {
			_, err = out.AddParseTree(name, t.Tree)
			if err != nil {
				return nil, err
			}
			continue
		}
		buf.Reset()
		err = markdownify(buf, t.Tree.Root)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		body := buf.String()
		_, err = out.New(name).Parse(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w\n%s", name, err, body)
		}
	}
	return out.Lookup(in.Name()), nil
}

func markdownify(buf *bytes.Buffer, node parse.Node) error {
	var err error
	switch node := node.(type) {
	case *parse.ListNode:
		for i := range node.Nodes {
			err = markdownify(buf, node.Nodes[i])
			if err != nil {
				return err
			}
		}
	case *parse.BranchNode:
		switch node.NodeType {
		case parse.NodeIf:
			buf.WriteString("{{if ")
		case parse.NodeWith:
			buf.WriteString("{{with ")
		default:
			panic("unknown branch type")
		}
		buf.WriteString(node.Pipe.String() + "}}")
		err = markdownify(buf, node.List)
		if err != nil {
			return err
		}
		if node.ElseList != nil {
			buf.WriteString("{{else}}")
			err = markdownify(buf, node.ElseList)
			if err != nil {
				return err
			}
		}
		buf.WriteString("{{end}}")
	case *parse.RangeNode:
		buf.WriteString("{{range " + node.Pipe.String() + "}}")
		err = markdownify(buf, node.List)
		if err != nil {
			return err
		}
		if node.ElseList != nil {
			buf.WriteString("{{else}}")
			err = markdownify(buf, node.ElseList)
			if err != nil {
				return err
			}
		}
		buf.WriteString("{{end}}")
	case *parse.TextNode:
		err = markdownConverter.Convert(node.Text, buf)
		if err != nil {
			return err
		}
	default:
		buf.WriteString(node.String())
	}
	return nil
}

var (
	templateQueries   map[string]func(*url.URL, ...string) (any, error)
	templateQueriesMu sync.RWMutex
)

func RegisterTemplateQuery(name string, query func(*url.URL, ...string) (any, error)) {
	templateQueriesMu.Lock()
	defer templateQueriesMu.Unlock()
	templateQueries[name] = query
}

var funcmap = map[string]any{
	"list": func(args ...any) []any { return args },
	"dict": func(args ...any) (map[string]any, error) {
		if len(args)%2 != 0 {
			return nil, fmt.Errorf("odd number of args")
		}
		var ok bool
		var key string
		dict := make(map[string]any)
		for i, arg := range args {
			if i%2 != 0 {
				key, ok = arg.(string)
				if !ok {
					return nil, fmt.Errorf("argument %#v is not a string", arg)
				}
				continue
			}
			dict[key] = arg
		}
		return dict, nil
	},
	"joinPath": path.Join,
	"prefix": func(s string, prefix string) string {
		if s == "" {
			return ""
		}
		return prefix + s
	},
	"suffix": func(s string, suffix string) string {
		if s == "" {
			return ""
		}
		return s + suffix
	},
	"json": func(v any) (string, error) {
		buf := bufpool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufpool.Put(buf)
		enc := json.NewEncoder(buf)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		err := enc.Encode(v)
		if err != nil {
			return "", err
		}
		return buf.String(), nil
	},
	"img": func(u *url.URL, src string, attrs ...string) (template.HTML, error) {
		var b strings.Builder
		b.WriteString("<img")
		src = html.EscapeString(src)
		if !strings.HasPrefix(src, "/") && !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
			src = path.Join(html.EscapeString(u.Path), src)
		}
		b.WriteString(` src="` + src + `"`)
		for _, attr := range attrs {
			name, value, _ := strings.Cut(attr, " ")
			b.WriteString(" " + html.EscapeString(name))
			if value != "" {
				b.WriteString(`="` + html.EscapeString(value) + `"`)
			}
		}
		b.WriteString(">")
		return template.HTML(b.String()), nil
	},
}

func FuncMap() map[string]any {
	queries := make(map[string]func(*url.URL, ...string) (any, error))
	templateQueriesMu.RLock()
	defer templateQueriesMu.RUnlock()
	for name, query := range templateQueries {
		queries[name] = query
	}
	m := make(map[string]any)
	for name, fn := range funcmap {
		m[name] = fn
	}
	m["query"] = func(name string, p *url.URL, args ...string) (any, error) {
		fn := queries[name]
		if fn == nil {
			return nil, fmt.Errorf("no such query %q", name)
		}
		return fn(p, args...)
	}
	m["hasQuery"] = func(name string) bool {
		fn := queries[name]
		return fn != nil
	}
	return m
}

type Funcs struct {
	fs      fs.FS
	queries map[string]func(*url.URL, ...string) (any, error)
}

type PageIndex struct {
	url.URL
	Pages []IndexEntry
}

type IndexEntry struct {
	url.URL
	Data map[string]string
}

func (f *Funcs) Index(u *url.URL, args ...string) (any, error) {
	domain, subdomain := splitHost(u.Host)
	tildePrefix, pathName := splitPath(u.Path)
	entries, err := fs.ReadDir(f.fs, path.Join(domain, subdomain, tildePrefix, "pm-route", pathName))
	if err != nil {
		return nil, err
	}
	index := &PageIndex{
		URL:   *u,
		Pages: make([]IndexEntry, len(entries)),
	}
	g, ctx := errgroup.WithContext(context.Background())
	for i, entry := range entries {
		i, entry := i, entry
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if !entry.IsDir() {
				return nil
			}
			dirname := entry.Name()
			filenames := []string{"index.html", "index.md"}
			var file fs.File
			for _, filename := range filenames {
				file, err = f.fs.Open(path.Join(domain, subdomain, tildePrefix, "pm-route", pathName, dirname, filename))
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				break
			}
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			fileinfo, err := file.Stat()
			if err != nil {
				return err
			}
			filename := fileinfo.Name()
			defer file.Close()
			buf := bufpool.Get().(*bytes.Buffer)
			buf.Reset()
			defer bufpool.Put(buf)
			_, err = buf.ReadFrom(file)
			if err != nil {
				return err
			}
			body := buf.String()
			t, err := template.New(filename).Funcs(FuncMap()).Parse(body)
			if err != nil {
				return err
			}
			if strings.HasSuffix(filename, ".md") {
				t, err = Markdownify(t, FuncMap())
				if err != nil {
					return err
				}
			}
			index.Pages[i].URL = *u
			index.Pages[i].URL.Path = path.Join(u.Path, dirname)
			index.Pages[i].Data = make(map[string]string)
			for _, t := range t.Templates() {
				name := t.Name()
				isDataTemplate := len(name) > 0 && unicode.IsUpper(rune(name[0]))
				if t.Tree != nil && isDataTemplate && filepath.Ext(name) == "" {
					index.Pages[i].Data[name] = t.Tree.Root.String()
				}
			}
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	n := 0
	for _, page := range index.Pages {
		if page.Data != nil {
			index.Pages[n] = page
			n++
		}
	}
	index.Pages = index.Pages[:n]
	return index, nil
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
	main, err := template.New(name).Funcs(FuncMap()).Parse(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if strings.HasSuffix(name, ".md") {
		main, err = Markdownify(main, FuncMap())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}

	visited := make(map[string]struct{})
	page := template.New("").Funcs(FuncMap())
	tmpls := main.Templates()
	var tmpl *template.Template
	var nodes []parse.Node
	var node parse.Node
	var errmsgs []string
	for len(tmpls) > 0 {
		tmpl, tmpls = tmpls[len(tmpls)-1], tmpls[:len(tmpls)-1]
		if tmpl.Tree == nil {
			continue
		}
		if cap(nodes) < len(tmpl.Tree.Root.Nodes) {
			nodes = make([]parse.Node, 0, len(tmpl.Tree.Root.Nodes))
		}
		for i := len(tmpl.Tree.Root.Nodes) - 1; i >= 0; i-- {
			nodes = append(nodes, tmpl.Tree.Root.Nodes[i])
		}
		for len(nodes) > 0 {
			node, nodes = nodes[len(nodes)-1], nodes[:len(nodes)-1]
			switch node := node.(type) {
			case *parse.ListNode:
				for i := len(node.Nodes) - 1; i >= 0; i-- {
					nodes = append(nodes, node.Nodes[i])
				}
			case *parse.BranchNode:
				nodes = append(nodes, node.List)
				if node.ElseList != nil {
					nodes = append(nodes, node.ElseList)
				}
			case *parse.RangeNode:
				nodes = append(nodes, node.List)
				if node.ElseList != nil {
					nodes = append(nodes, node.ElseList)
				}
			case *parse.TemplateNode:
				if !strings.HasSuffix(node.Name, ".html") && !strings.HasSuffix(node.Name, ".md") {
					continue
				}
				if _, ok := visited[node.Name]; ok {
					continue
				}
				visited[node.Name] = struct{}{}
				file, err := pm.fs.Open(path.Join("pm-template", node.Name))
				if err != nil {
					body := tmpl.Tree.Root.String()
					pos := int(node.Position())
					line := 1 + strings.Count(body[:pos], "\n")
					if errors.Is(err, fs.ErrNotExist) {
						errmsgs = append(errmsgs, fmt.Sprintf("%s line %d: %s does not exist", tmpl.Name(), line, node.String()))
						continue
					}
					return nil, fmt.Errorf("%s line %d: %s: %w", tmpl.Name(), line, node.String(), err)
				}
				buf.Reset()
				_, err = buf.ReadFrom(file)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", node.Name, err)
				}
				body := buf.String()
				t, err := template.New(node.Name).Funcs(FuncMap()).Parse(body)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", node.Name, err)
				}
				if strings.HasSuffix(node.Name, ".md") {
					t, err = Markdownify(t, FuncMap())
					if err != nil {
						return nil, fmt.Errorf("%s: %w", node.Name, err)
					}
				}
				for _, t := range t.Templates() {
					_, err = page.AddParseTree(t.Name(), t.Tree)
					if err != nil {
						return nil, fmt.Errorf("%s: adding %s: %w", node.Name, t.Name(), err)
					}
					tmpls = append(tmpls, t)
				}
			}
		}
	}
	if len(errmsgs) > 0 {
		return nil, fmt.Errorf("invalid template references:\n" + strings.Join(errmsgs, "\n"))
	}

	for _, t := range main.Templates() {
		_, err = page.AddParseTree(t.Name(), t.Tree)
		if err != nil {
			return nil, fmt.Errorf("%s: adding %s: %w", name, t.Name(), err)
		}
	}
	page = page.Lookup(name)
	return page, nil
}

func (pm *Pagemanager) Error(w http.ResponseWriter, r *http.Request, msg string, code int) {
	statusCode := strconv.Itoa(code)
	errmsg := statusCode + " " + http.StatusText(code) + "\n\n" + msg
	domain, subdomain := splitHost(r.Host)
	tildePrefix, _ := splitPath(r.URL.Path)
	name := path.Join(domain, subdomain, tildePrefix, "pm-template", statusCode+".html")
	file, err := pm.fs.Open(name)
	if err != nil {
		http.Error(w, errmsg, code)
		return
	}
	defer file.Close()
	tmpl, err := pm.Template(name, file)
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	err = tmpl.ExecuteTemplate(buf, name, map[string]any{
		"URL": r.URL,
		"Msg": msg,
	})
	if err != nil {
		http.Error(w, errmsg+"\n\n(error executing "+name+": "+err.Error()+")", code)
		return
	}
	w.WriteHeader(code)
	http.ServeContent(w, r, statusCode+".html", time.Time{}, bytes.NewReader(buf.Bytes()))
}

func (pm *Pagemanager) NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pm.Error(w, r, path.Join(r.Host, r.URL.String()), 404)
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

	if filepath.Ext(name) != "" {
		file, err = pm.fs.Open(name)
		if err != nil {
			return nil, err
		}
		fileinfo, err := file.Stat()
		if err != nil {
			return nil, err
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer file.Close()
			fileSeeker, ok := file.(io.ReadSeeker)
			if !ok {
				w.Header().Set("Content-Type", mime.TypeByExtension(fileinfo.Name()))
				w.Header().Set("X-Content-Type-Options", "nosniff")
				_, _ = io.Copy(w, file)
				return
			}
			http.ServeContent(w, r, fileinfo.Name(), fileinfo.ModTime(), fileSeeker)
		}), nil
	}

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
	modtime := fileinfo.ModTime()
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

	page, err := pm.Template(handlerPath, file)
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := bufpool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufpool.Put(buf)
		if data == nil {
			data = make(map[string]any)
		}
		data["URL"] = r.URL
		err = page.ExecuteTemplate(buf, handlerPath, data)
		if err != nil {
			fmt.Println(buf.String())
			pm.InternalServerError(err).ServeHTTP(w, r)
			return
		}
		http.ServeContent(w, r, filename, modtime, bytes.NewReader(buf.Bytes()))
	}), nil
}

func (pm *Pagemanager) Static(w http.ResponseWriter, r *http.Request, name string) {
	if !strings.HasPrefix(name, "pm-static") {
		name = path.Join("pm-static", name)
	}
	name = strings.TrimPrefix(name, "/")
	names := make([]string, 0, 2)
	names = append(names, name)
	if strings.HasPrefix(name, "pm-static/pm-template") {
		names = append(names, strings.TrimPrefix(name, "pm-static/"))
	}
	var err error
	var file fs.File
	for _, name := range names {
		file, err = pm.fs.Open(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err == nil {
			break
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		pm.NotFound().ServeHTTP(w, r)
		return
	}
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	fileinfo, err := file.Stat()
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	if fileinfo.IsDir() {
		pm.NotFound().ServeHTTP(w, r)
		return
	}
	fileSeeker, ok := file.(io.ReadSeeker)
	if !ok {
		w.Header().Set("Content-Type", mime.TypeByExtension(fileinfo.Name()))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = io.Copy(w, file)
		return
	}
	http.ServeContent(w, r, fileinfo.Name(), fileinfo.ModTime(), fileSeeker)
}

func (pm *Pagemanager) debug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	err := r.ParseForm()
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	buf := bufpool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufpool.Put(buf)
	filename := r.Form.Get("f")
	templateName := r.Form.Get("t")
	step := r.Form.Get("s")
	file, err := pm.fs.Open(path.Join("pm-route", filename))
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	defer file.Close()
	_, err = buf.ReadFrom(file)
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	body := buf.String()
	t, err := template.New(filename).Parse(body)
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	var b strings.Builder
	if err := markdownConverter.Convert(buf.Bytes(), &b); err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	fmt.Println(b.String())

	if templateName != "" {
		t = t.Lookup(templateName)
		if t == nil {
			pm.Error(w, r, "no such template "+templateName, 400)
			return
		}
	}
	if t.Tree == nil {
		pm.Error(w, r, t.Name()+" is empty", 500)
		return
	}
	if step == "" || step == "0" {
		_, _ = io.WriteString(w, t.Tree.Root.String())
		return
	}

	t, err = Markdownify(t, FuncMap())
	if err != nil {
		pm.InternalServerError(err).ServeHTTP(w, r)
		return
	}
	if step == "1" {
		_, _ = io.WriteString(w, t.Tree.Root.String())
		return
	}

	err = t.ExecuteTemplate(w, templateName, map[string]any{
		"URL": r.URL,
	})
	if err != nil {
		_, _ = io.WriteString(w, "\n\n"+err.Error())
	}
}

func (pm *Pagemanager) Pagemanager(next http.Handler) http.Handler {
	pm.wfs, _ = pm.fs.(WriteableFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain, subdomain := splitHost(r.Host)
		tildePrefix, pathName := splitPath(r.URL.Path)
		// pm-debug.
		if pathName == "pm-debug" || strings.HasPrefix(pathName, "pm-debug/") {
			pm.debug(w, r)
			return
		}
		// pm-static.
		if pathName == "pm-static" || strings.HasPrefix(pathName, "pm-static/") {
			pm.Static(w, r, pathName)
			return
		}
		// pm-site.
		// pm-route.
		name := path.Join(domain, subdomain, tildePrefix, "pm-route", pathName)
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
	if host == "localhost" || strings.HasPrefix(host, "localhost:") || host == "127.0.0.1" || strings.HasPrefix(host, "127.0.0.1:") {
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

func splitPath(rawPath string) (tildePrefix, pathName string) {
	pathName = strings.TrimPrefix(rawPath, "/")
	if strings.HasPrefix(pathName, "~") {
		if i := strings.Index(pathName, "/"); i >= 0 {
			tildePrefix, pathName = pathName[:i], pathName[i+1:]
		}
	}
	return tildePrefix, pathName
}
