package parser

import (
	"reflect"
	"strconv"
	"strings"

	"project/external/anko/ast"
)

// Scanner stores information for lexer.
type Scanner struct {
	src      []rune
	offset   int
	lineHead int
	line     int
}

var (
	nilValue   = reflect.New(reflect.TypeOf((*interface{})(nil)).Elem()).Elem()
	trueValue  = reflect.ValueOf(true)
	falseValue = reflect.ValueOf(false)
	oneLiteral = &ast.LiteralExpr{Literal: reflect.ValueOf(int64(1))}
)

// Parse provides way to parse the code using Scanner.
func Parse(s *Scanner) (ast.Stmt, error) {
	return nil, nil
}

// ParseSrc provides way to parse the code from source.
func ParseSrc(src string) (ast.Stmt, error) {
	scanner := &Scanner{
		src: []rune(src),
	}
	return Parse(scanner)
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
