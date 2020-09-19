package parser

import (
	"project/external/anko/ast"
)

// Scanner stores information for lexer.
type Scanner struct {
	src      []rune
	offset   int
	lineHead int
	line     int
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
