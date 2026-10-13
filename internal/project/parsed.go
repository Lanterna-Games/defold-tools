package project

import (
	"bytes"
	"sync"

	"github.com/Lanterna-Games/defold-tools/internal/lua"
	"github.com/Lanterna-Games/defold-tools/internal/textproto"
)

type parsedScript struct {
	content      []byte
	chunk        *lua.Chunk
	isReadInFull bool
}

type parsedDocument struct {
	content      []byte
	document     *textproto.Message
	syntaxErrors []textproto.SyntaxError
}

type Parsed struct {
	guard     sync.Mutex
	scripts   map[string]parsedScript
	documents map[string]parsedDocument
	Reused    int
	ReadAnew  int
}

func NewParsed() *Parsed {
	return &Parsed{scripts: map[string]parsedScript{}, documents: map[string]parsedDocument{}}
}

func (p *Parsed) script(filePath string, content []byte, parse func() (*lua.Chunk, bool)) (chunk *lua.Chunk, isReadInFull bool) {
	if p == nil {
		return parse()
	}
	p.guard.Lock()
	defer p.guard.Unlock()
	if known, isKnown := p.scripts[filePath]; isKnown && bytes.Equal(known.content, content) {
		p.Reused++
		return known.chunk, known.isReadInFull
	}
	chunk, isReadInFull = parse()
	p.ReadAnew++
	p.scripts[filePath] = parsedScript{content: content, chunk: chunk, isReadInFull: isReadInFull}
	return chunk, isReadInFull
}

func (p *Parsed) document(filePath string, content []byte, parse func() (*textproto.Message, []textproto.SyntaxError)) (*textproto.Message, []textproto.SyntaxError) {
	if p == nil {
		return parse()
	}
	p.guard.Lock()
	defer p.guard.Unlock()
	if known, isKnown := p.documents[filePath]; isKnown && bytes.Equal(known.content, content) {
		p.Reused++
		return known.document, known.syntaxErrors
	}
	document, syntaxErrors := parse()
	p.ReadAnew++
	p.documents[filePath] = parsedDocument{content: content, document: document, syntaxErrors: syntaxErrors}
	return document, syntaxErrors
}

func (p *Parsed) forgetAllBut(isThere map[string]bool) {
	if p == nil {
		return
	}
	p.guard.Lock()
	defer p.guard.Unlock()
	for filePath := range p.scripts {
		if !isThere["/"+filePath] {
			delete(p.scripts, filePath)
		}
	}
	for filePath := range p.documents {
		if !isThere["/"+filePath] {
			delete(p.documents, filePath)
		}
	}
}
