{{ $index := query "github.com/pagemanager/pagemanager.Funcs.Index" .URL }}
{{ range $page := $index.Pages }}
    {{ $page }}
{{ end }}
