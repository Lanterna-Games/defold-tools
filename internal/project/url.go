package project

import "strings"

type URL struct {
	Socket      string
	Path        string
	Fragment    string
	HasFragment bool
}

func ParseURL(text string) URL {
	var url URL
	if socket, rest, hasSocket := strings.Cut(text, ":"); hasSocket {
		url.Socket, text = socket, rest
	}
	url.Path, url.Fragment, url.HasFragment = strings.Cut(text, "#")
	return url
}

func (u URL) IsAbsolute() bool {
	return strings.HasPrefix(u.Path, "/")
}
