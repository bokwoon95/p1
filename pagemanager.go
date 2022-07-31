package pagemanager

import (
	"fmt"
	"path"
	"strings"
)

type Path struct {
	Domain      string
	Subdomain   string
	TildePrefix string
	Name        string // must start with pm-src, pm-site, pm-static or pm-template
}

// "localhost:8080", "/about-me" -> "pm-site/about-me/index.html"

// The pagemanager handler itself will handle converting r.Host and r.Path into a name

// string -> ParsedName (for filesystems that need to implement FS)

func NewPath(name string) (Path, error) {
	var p Path
	isValidName := func(name string) bool {
		return strings.HasPrefix(name, "pm-site/") ||
			strings.HasPrefix(name, "pm-src/") ||
			strings.HasPrefix(name, "pm-static/") ||
			strings.HasPrefix(name, "pm-template/")
	}
	if isValidName(name) {
		p.Name = name
		return p, nil
	}
	i := strings.Index(name, "/")
	if i < 0 {
		return p, fmt.Errorf("invalid name")
	}
	if strings.Index(name[:i], ".") < 0 {
		return p, fmt.Errorf("invalid name")
	}
	p.Domain = name[:i]
	if isValidName(name[i+1:]) {
		p.Name = name[i+1:]
		return p, nil
	}
	j := strings.Index(name[i+1:], "/")
	if j < 0 {
		return p, fmt.Errorf("invalid name")
	}
	if strings.HasPrefix(name[i+1:j], "~") {
	}
	return p, nil
}

func (p Path) String() string {
	return path.Join(p.Domain, p.Subdomain, p.TildePrefix, p.Name)
}

type FileRouter struct{ // Either return index.html, index.md or handler.txt.
}
