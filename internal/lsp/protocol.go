package lsp

const (
	severityError    = 1
	severityWarning  = 2
	syncFullText     = 1
	messageIsError   = 1
	messageIsWarning = 2
	messageIsInfo    = 3
	sourceOfOurs     = "defold-vet"
)

type initializeParams struct {
	RootURI          string            `json:"rootUri"`
	RootPath         string            `json:"rootPath"`
	WorkspaceFolders []workspaceFolder `json:"workspaceFolders"`
}

type workspaceFolder struct {
	URI string `json:"uri"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	TextDocumentSync   textDocumentSync   `json:"textDocumentSync"`
	CompletionProvider *completionOptions `json:"completionProvider,omitempty"`
	DefinitionProvider bool               `json:"definitionProvider,omitempty"`
	HoverProvider      bool               `json:"hoverProvider,omitempty"`
	CodeActionProvider bool               `json:"codeActionProvider,omitempty"`
	ReferencesProvider bool               `json:"referencesProvider,omitempty"`
	RenameProvider     *renameOptions     `json:"renameProvider,omitempty"`
	Workspace          *workspaceOptions  `json:"workspace,omitempty"`
}

type workspaceOptions struct {
	FileOperations fileOperations `json:"fileOperations"`
}

type fileOperations struct {
	WillRename fileOperationRegistration `json:"willRename"`
}

type fileOperationRegistration struct {
	Filters []fileOperationFilter `json:"filters"`
}

type fileOperationFilter struct {
	Scheme  string               `json:"scheme"`
	Pattern fileOperationPattern `json:"pattern"`
}

type fileOperationPattern struct {
	Glob string `json:"glob"`
}

type renameFilesParams struct {
	Files []renamedFile `json:"files"`
}

type renamedFile struct {
	OldURI string `json:"oldUri"`
	NewURI string `json:"newUri"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type positionParams struct {
	TextDocument textDocument `json:"textDocument"`
	Position     position     `json:"position"`
	Range        textRange    `json:"range"`
	NewName      string       `json:"newName"`
}

type renameOptions struct {
	PrepareProvider bool `json:"prepareProvider"`
}

type whatCanBeRenamed struct {
	Range       textRange `json:"range"`
	Placeholder string    `json:"placeholder"`
}

type completionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind"`
	Detail        string         `json:"detail,omitempty"`
	Documentation *markupContent `json:"documentation,omitempty"`
	FilterText    string         `json:"filterText"`
	SortText      string         `json:"sortText"`
	TextEdit      textEdit       `json:"textEdit"`
}

type textEdit struct {
	Range   textRange `json:"range"`
	NewText string    `json:"newText"`
}

type location struct {
	URI   string    `json:"uri"`
	Range textRange `json:"range"`
}

type hover struct {
	Contents markupContent `json:"contents"`
	Range    textRange     `json:"range"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type codeAction struct {
	Title       string        `json:"title"`
	Kind        string        `json:"kind"`
	Diagnostics []diagnostic  `json:"diagnostics"`
	IsPreferred bool          `json:"isPreferred"`
	Edit        workspaceEdit `json:"edit"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

type textDocumentSync struct {
	OpenClose bool        `json:"openClose"`
	Change    int         `json:"change"`
	Save      saveOptions `json:"save"`
}

type saveOptions struct {
	IncludeText bool `json:"includeText"`
}

type textDocument struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocument `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument   textDocument    `json:"textDocument"`
	ContentChanges []contentChange `json:"contentChanges"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didCloseParams struct {
	TextDocument textDocument `json:"textDocument"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type textRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type diagnostic struct {
	Range    textRange `json:"range"`
	Severity int       `json:"severity"`
	Code     string    `json:"code"`
	Source   string    `json:"source"`
	Message  string    `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type showMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}
