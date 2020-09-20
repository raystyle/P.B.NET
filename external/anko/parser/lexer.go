package parser

import (
	"reflect"
	"strconv"
	"strings"

	"project/external/anko/ast"
)

var (
	nilValue   = reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem()
	trueValue  = reflect.ValueOf(true)
	falseValue = reflect.ValueOf(false)
	oneLiteral = &ast.LiteralExpr{Literal: reflect.ValueOf(int64(1))}
)

// Error is a parse error.
type Error struct {
	Message  string
	Pos      ast.Position
	Filename string
	Fatal    bool
}

// Error returns the parse error message.
func (e *Error) Error() string {
	return e.Message
}

// Lexer provides interface to parse codes.
type Lexer struct {
	s    *Scanner
	lit  string
	pos  ast.Position
	e    error
	stmt ast.Stmt
}

// Lex scans the token and literals.
func (l *Lexer) Lex(val *yySymType) int {
	tok, lit, pos, err := l.s.Scan()
	if err != nil {
		l.e = &Error{Message: err.Error(), Pos: pos, Fatal: true}
	}
	val.tok = ast.Token{Tok: tok, Lit: lit}
	val.tok.SetPosition(pos)
	l.lit = lit
	l.pos = pos
	return tok
}

// Error sets parse error.
func (l *Lexer) Error(msg string) {
	l.e = &Error{Message: msg, Pos: l.pos, Fatal: false}
}

// Parse provides way to parse the code using Scanner.
func Parse(s *Scanner) (ast.Stmt, error) {
	l := Lexer{s: s}
	if yyParse(&l) != 0 {
		return nil, l.e
	}
	return l.stmt, l.e
}

// ParseSrc provides way to parse the code from source.
func ParseSrc(src string) (ast.Stmt, error) {
	scanner := &Scanner{
		src: []rune(src),
	}
	return Parse(scanner)
}

// EnableErrorVerbose enabled verbose errors from the parser.
func EnableErrorVerbose() {
	yyErrorVerbose = true
}

// EnableDebug enabled debug from the parser.
func EnableDebug(level int) {
	yyDebug = level
}

func toNumber(numString string) (reflect.Value, error) {
	// hex
	if len(numString) > 2 && numString[0:2] == "0x" {
		i, err := strconv.ParseInt(numString[2:], 16, 64)
		if err != nil {
			return nilValue, err
		}
		return reflect.ValueOf(i), nil
	}

	// hex
	if len(numString) > 3 && numString[0:3] == "-0x" {
		i, err := strconv.ParseInt("-"+numString[3:], 16, 64)
		if err != nil {
			return nilValue, err
		}
		return reflect.ValueOf(i), nil
	}

	// float
	if strings.Contains(numString, ".") || strings.Contains(numString, "e") {
		f, err := strconv.ParseFloat(numString, 64)
		if err != nil {
			return nilValue, err
		}
		return reflect.ValueOf(f), nil
	}

	// int
	i, err := strconv.ParseInt(numString, 10, 64)
	if err != nil {
		return nilValue, err
	}
	return reflect.ValueOf(i), nil
}

func stringToValue(aString string) reflect.Value {
	return reflect.ValueOf(aString)
}
