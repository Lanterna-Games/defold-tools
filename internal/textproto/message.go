package textproto

import "github.com/Lanterna-Games/defold-tools/internal/source"

type Message struct {
	Fields []Field
}

type Field struct {
	Name     string
	Position source.Position
	Scalar   *Scalar
	Message  *Message
}

func (m *Message) Messages(name string) []*Message {
	var found []*Message
	for _, field := range m.Fields {
		if field.Name == name && field.Message != nil {
			found = append(found, field.Message)
		}
	}
	return found
}

func (m *Message) Scalars(name string) []*Scalar {
	var found []*Scalar
	for _, field := range m.Fields {
		if field.Name == name && field.Scalar != nil {
			found = append(found, field.Scalar)
		}
	}
	return found
}

func (m *Message) Scalar(name string) (scalar *Scalar, found bool) {
	scalars := m.Scalars(name)
	if len(scalars) == 0 {
		return nil, false
	}
	return scalars[0], true
}

func (m *Message) Text(name string) string {
	scalar, found := m.Scalar(name)
	if !found {
		return ""
	}
	return scalar.Text
}
