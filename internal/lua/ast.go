package lua

import "github.com/Lanterna-Games/defold-tools/internal/source"

type Range struct {
	Start source.Position
	End   source.Position
}

func (r Range) Span() Range {
	return r
}

type Node interface {
	Span() Range
}

type Expression interface {
	Node
	expression()
}

type Statement interface {
	Node
	statement()
}

type Chunk struct {
	Range
	Body     []Statement
	Comments []Comment
}

type (
	NilLiteral     struct{ Range }
	VarargLiteral  struct{ Range }
	BooleanLiteral struct {
		Range
		Value bool
	}
	NumberLiteral struct {
		Range
		Text string
	}
	StringLiteral struct {
		Range
		Value string
	}
	Identifier struct {
		Range
		Name string
	}
	FunctionLiteral struct {
		Range
		Parameters []*Identifier
		IsVararg   bool
		Body       []Statement
	}
	IndexExpression struct {
		Range
		Object Expression
		Index  Expression
	}
	CallExpression struct {
		Range
		Function  Expression
		Method    *Identifier
		Arguments []Expression
	}
	BinaryExpression struct {
		Range
		Operator string
		Left     Expression
		Right    Expression
	}
	UnaryExpression struct {
		Range
		Operator string
		Operand  Expression
	}
	ParenthesizedExpression struct {
		Range
		Inner Expression
	}
	TableLiteral struct {
		Range
		Fields []TableField
	}
	BadExpression struct{ Range }
)

type TableField struct {
	Key   Expression
	Value Expression
}

func (*NilLiteral) expression()              {}
func (*VarargLiteral) expression()           {}
func (*BooleanLiteral) expression()          {}
func (*NumberLiteral) expression()           {}
func (*StringLiteral) expression()           {}
func (*Identifier) expression()              {}
func (*FunctionLiteral) expression()         {}
func (*IndexExpression) expression()         {}
func (*CallExpression) expression()          {}
func (*BinaryExpression) expression()        {}
func (*UnaryExpression) expression()         {}
func (*ParenthesizedExpression) expression() {}
func (*TableLiteral) expression()            {}
func (*BadExpression) expression()           {}

type (
	LocalStatement struct {
		Range
		Names  []*Identifier
		Values []Expression
	}
	AssignmentStatement struct {
		Range
		Targets []Expression
		Values  []Expression
	}
	CallStatement struct {
		Range
		Call *CallExpression
	}
	DoStatement struct {
		Range
		Body []Statement
	}
	WhileStatement struct {
		Range
		Condition Expression
		Body      []Statement
	}
	RepeatStatement struct {
		Range
		Body      []Statement
		Condition Expression
	}
	IfStatement struct {
		Range
		Clauses   []IfClause
		Else      []Statement
		HasElse   bool
		ElseStart source.Position
	}
	NumericForStatement struct {
		Range
		Variable *Identifier
		Start    Expression
		Stop     Expression
		Step     Expression
		Body     []Statement
	}
	GenericForStatement struct {
		Range
		Names  []*Identifier
		Values []Expression
		Body   []Statement
	}
	FunctionStatement struct {
		Range
		Name     Expression
		Method   *Identifier
		Function *FunctionLiteral
	}
	LocalFunctionStatement struct {
		Range
		Name     *Identifier
		Function *FunctionLiteral
	}
	ReturnStatement struct {
		Range
		Values []Expression
	}
	BreakStatement struct{ Range }
)

type IfClause struct {
	Start     source.Position
	Condition Expression
	Body      []Statement
}

func (*LocalStatement) statement()         {}
func (*AssignmentStatement) statement()    {}
func (*CallStatement) statement()          {}
func (*DoStatement) statement()            {}
func (*WhileStatement) statement()         {}
func (*RepeatStatement) statement()        {}
func (*IfStatement) statement()            {}
func (*NumericForStatement) statement()    {}
func (*GenericForStatement) statement()    {}
func (*FunctionStatement) statement()      {}
func (*LocalFunctionStatement) statement() {}
func (*ReturnStatement) statement()        {}
func (*BreakStatement) statement()         {}
