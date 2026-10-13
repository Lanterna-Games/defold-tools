package project

import "github.com/Lanterna-Games/defold-tools/internal/lua"

const propertyDeclarationFunction = "go.property"

func parsedLua(content []byte) (chunk *lua.Chunk, isReadInFull bool) {
	chunk, syntaxErrors := lua.Parse(content)
	return chunk, len(syntaxErrors) == 0
}

func scriptFrom(filePath, resourcePath string, content []byte) *Script {
	chunk, isReadInFull := parsedLua(content)
	return scriptReadAs(filePath, resourcePath, content, chunk, isReadInFull)
}

func scriptReadAs(filePath, resourcePath string, content []byte, chunk *lua.Chunk, isReadInFull bool) *Script {
	constants := constantsOf(chunk)
	tableFields := tableFieldsOf(chunk, constants)
	script := &Script{
		Path:                        resourcePath,
		File:                        filePath,
		Chunk:                       chunk,
		Source:                      content,
		IsReadInFull:                isReadInFull,
		DeclaresPropertiesByLiteral: true,
		constants:                   constants,
		tableFields:                 tableFields,
		exports:                     exportsOf(chunk, tableFields),
	}
	lua.Inspect(chunk, func(node lua.Node) bool {
		call, isCall := node.(*lua.CallExpression)
		if !isCall {
			return true
		}
		if function, _ := lua.QualifiedName(call.Function); function != propertyDeclarationFunction {
			return true
		}
		if len(call.Arguments) == 0 {
			return true
		}
		name, isLiteral := call.Arguments[0].(*lua.StringLiteral)
		if !isLiteral {
			script.DeclaresPropertiesByLiteral = false
			script.cannotRead(call.Arguments[0], "the name of the property declared here cannot be read, so the properties of this script are not judged")
			return true
		}
		script.Properties = append(script.Properties, PropertyDeclaration{
			Name:     name.Value,
			Location: Location{File: filePath, Position: name.Start},
		})
		return true
	})
	return script
}

func (s *Script) readWithTheModulesOf(loaded *Project) {
	s.moduleAt = func(resourcePath string) (module *Script, isRead bool) {
		module, isRead = loaded.Scripts[resourcePath]
		return module, isRead
	}
	s.collectMessages()
	s.collectActions()
	s.collectGUINodes()
}
