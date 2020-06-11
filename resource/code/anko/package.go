package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func isGoFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	name := info.Name()
	if name == "fuzz.go" {
		return false
	}
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	if strings.HasPrefix(name, "example_") {
		return false
	}
	return true
}

func parseDir(dir string) (map[string]*ast.Package, error) {
	return parser.ParseDir(token.NewFileSet(), dir, isGoFile, parser.ParseComments)
}

func getPackageName(packages map[string]*ast.Package) string {
	var pkgName string
	for pn := range packages {
		switch {
		case pn == "main":
		case strings.HasSuffix(pn, "_test"):
		default:
			pkgName = pn
			break
		}
	}
	return pkgName
}

func isDeprecated(text string) bool {
	return strings.Contains(text, "Deprecated:")
}

func exportValues(decl *ast.GenDecl, m map[string]struct{}) {
	if isDeprecated(decl.Doc.Text()) {
		return
	}
	for _, spec := range decl.Specs {
		vs := spec.(*ast.ValueSpec)
		if isDeprecated(vs.Doc.Text()) {
			continue
		}
		for _, name := range vs.Names {
			// filter some variables
			if name.Name == "ErrTrailingComma" {
				continue
			}
			if name.IsExported() {
				m[name.Name] = struct{}{}
			}
		}
	}
}

func exportTypes(decl *ast.GenDecl, m map[string]struct{}) {
	if isDeprecated(decl.Doc.Text()) {
		return
	}
	for _, spec := range decl.Specs {
		ts := spec.(*ast.TypeSpec)
		if isDeprecated(ts.Doc.Text()) {
			continue
		}
		if ts.Name.IsExported() {
			m[ts.Name.Name] = struct{}{}
		}
	}
}

func exportFunction(decl *ast.FuncDecl, m map[string]struct{}) {
	if isDeprecated(decl.Doc.Text()) {
		return
	}
	if decl.Recv != nil {
		return
	}
	if decl.Name.IsExported() {
		m[decl.Name.Name] = struct{}{}
	}
}

func exportDeclaration(root, path, init string) (string, error) {
	// package file path
	packages, err := parseDir(filepath.Join(root, path))
	if err != nil {
		return "", err
	}
	pkgName := getPackageName(packages)
	// walk files
	constants := make(map[string]struct{})
	variables := make(map[string]struct{})
	types := make(map[string]struct{})
	functions := make(map[string]struct{})
	for _, file := range packages[pkgName].Files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				switch decl.Tok {
				case token.CONST:
					exportValues(decl, constants)
				case token.VAR:
					exportValues(decl, variables)
				case token.TYPE:
					exportTypes(decl, types)
				}
			case *ast.FuncDecl:
				exportFunction(decl, functions)
			}
		}
	}
	// replace special path
	if root[0] == '$' {

	}

	return generateCode(path, pkgName, init, constants, variables, types, functions), nil
}

// pn = package name
func generateCode(path, pn, init string, consts, vars, types, fns map[string]struct{}) string {
	const template = `
func init%s() {
	env.Packages["%s"] = map[string]reflect.Value{
		// define constants
%s
		// define variables
%s
		// define functions
%s	}
	var (
%s	)
	env.PackageTypes["%s"] = map[string]reflect.Type{
%s	}
}
`
	const (
		tabs = "\t\t"

		// "Compare": reflect.ValueOf(bytes.Compare),
		valFormat = tabs + `"%s": reflect.ValueOf(%s.%s),` + "\n"

		// conn net.Conn
		vpFormat = tabs + "%s %s.%s\n"

		// "Conn": reflect.TypeOf(&conn).Elem(),
		typeFormat = tabs + `"%s": reflect.TypeOf(&%s).Elem(),` + "\n"
	)

	buf := new(bytes.Buffer)
	for c := range consts {
		_, _ = fmt.Fprintf(buf, valFormat, c, pn, c)
	}
	constStr := buf.String()

	buf.Reset()
	for v := range vars {
		_, _ = fmt.Fprintf(buf, valFormat, v, pn, v)
	}
	varStr := buf.String()

	buf.Reset()
	for fn := range fns {
		_, _ = fmt.Fprintf(buf, valFormat, fn, pn, fn)
	}
	funcStr := buf.String()

	// prepare var buffer for struct and interface
	vpBuf := new(bytes.Buffer)
	buf.Reset()
	for typ := range types {
		// "ReadWriter" -> "readWriter"
		varName := strings.ToLower(typ[0:1]) + typ[1:]
		_, _ = fmt.Fprintf(vpBuf, vpFormat, varName, pn, typ)
		_, _ = fmt.Fprintf(buf, typeFormat, typ, varName)
	}
	vpStr := vpBuf.String()
	typeStr := buf.String()
	return fmt.Sprintf(template, init, path, constStr, varStr, funcStr, vpStr, path, typeStr)
}
