{{ $index := query "github.com/pagemanager/pagemanager.Funcs.Index" . }}
{{ range $page := $index.Pages }}
    {{ $title := $page.Data.Title }}
    {{ if not $title }}{{ $title = $page.Path }}{{ end }}
    - [{{ $title }}]({{ $page.Path }})
        {{ with $page.Data.Summary }}
        - {{ . }}
        {{ end }}
{{ end }}
