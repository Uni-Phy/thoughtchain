package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// CodeStructure represents the analyzed code structure
type CodeStructure struct {
	Files     map[string]*FileInfo `json:"files"`
	Functions map[string]*FuncInfo `json:"functions"`
	Types     map[string]*TypeInfo `json:"types"`
}

// FileInfo represents information about a source file
type FileInfo struct {
	Path     string   `json:"path"`
	Package  string   `json:"package"`
	Imports  []string `json:"imports"`
	Contents string   `json:"contents"`
}

// FuncInfo represents information about a function
type FuncInfo struct {
	Name       string    `json:"name"`
	File       string    `json:"file"`
	Package    string    `json:"package"`
	Signature  string    `json:"signature"`
	Doc        string    `json:"doc"`
	StartLine  int       `json:"start_line"`
	EndLine    int       `json:"end_line"`
	Complexity int       `json:"complexity"`
	Calls      []string  `json:"calls"`
	CalledBy   []string  `json:"called_by"`
}

// TypeInfo represents information about a type
type TypeInfo struct {
	Name       string    `json:"name"`
	File       string    `json:"file"`
	Package    string    `json:"package"`
	Doc        string    `json:"doc"`
	Fields     []string  `json:"fields"`
	Methods    []string  `json:"methods"`
	Implements []string  `json:"implements"`
}

// Analyzer handles code analysis
type Analyzer struct {
	fset      *token.FileSet
	structure *CodeStructure
}

// NewAnalyzer creates a new code analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		fset: token.NewFileSet(),
		structure: &CodeStructure{
			Files:     make(map[string]*FileInfo),
			Functions: make(map[string]*FuncInfo),
			Types:     make(map[string]*TypeInfo),
		},
	}
}

// AnalyzeDirectory analyzes all Go files in a directory
func (a *Analyzer) AnalyzeDirectory(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			if err := a.analyzeFile(path); err != nil {
				return fmt.Errorf("failed to analyze %s: %w", path, err)
			}
		}
		return nil
	})
}

// analyzeFile analyzes a single Go file
func (a *Analyzer) analyzeFile(path string) error {
	// Parse the file
	node, err := parser.ParseFile(a.fset, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	// Read file contents
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Create file info
	fileInfo := &FileInfo{
		Path:     path,
		Package:  node.Name.Name,
		Contents: string(contents),
	}

	// Extract imports
	for _, imp := range node.Imports {
		if imp.Path != nil {
			fileInfo.Imports = append(fileInfo.Imports, strings.Trim(imp.Path.Value, "\""))
		}
	}

	// Store file info
	a.structure.Files[path] = fileInfo

	// Analyze declarations
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			a.analyzeFuncDecl(x, path, node.Name.Name)
		case *ast.TypeSpec:
			a.analyzeTypeSpec(x, path, node.Name.Name)
		}
		return true
	})

	return nil
}

// analyzeFuncDecl analyzes a function declaration
func (a *Analyzer) analyzeFuncDecl(fn *ast.FuncDecl, file, pkg string) {
	name := fn.Name.Name
	if fn.Recv != nil {
		// Method
		if len(fn.Recv.List) > 0 {
			switch t := fn.Recv.List[0].Type.(type) {
			case *ast.StarExpr:
				if ident, ok := t.X.(*ast.Ident); ok {
					name = fmt.Sprintf("%s.%s", ident.Name, name)
				}
			case *ast.Ident:
				name = fmt.Sprintf("%s.%s", t.Name, name)
			}
		}
	}

	funcInfo := &FuncInfo{
		Name:      name,
		File:      file,
		Package:   pkg,
		Doc:       fn.Doc.Text(),
		StartLine: a.fset.Position(fn.Pos()).Line,
		EndLine:   a.fset.Position(fn.End()).Line,
	}

	// Extract function calls
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				funcInfo.Calls = append(funcInfo.Calls, fmt.Sprintf("%s.%s", sel.X, sel.Sel.Name))
			}
		}
		return true
	})

	a.structure.Functions[fmt.Sprintf("%s.%s", pkg, name)] = funcInfo
}

// analyzeTypeSpec analyzes a type declaration
func (a *Analyzer) analyzeTypeSpec(typ *ast.TypeSpec, file, pkg string) {
	typeInfo := &TypeInfo{
		Name:    typ.Name.Name,
		File:    file,
		Package: pkg,
		Doc:     typ.Doc.Text(),
	}

	// Extract fields for structs
	if st, ok := typ.Type.(*ast.StructType); ok {
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				typeInfo.Fields = append(typeInfo.Fields, name.Name)
			}
		}
	}

	// Extract interface methods
	if iface, ok := typ.Type.(*ast.InterfaceType); ok {
		for _, method := range iface.Methods.List {
			for _, name := range method.Names {
				typeInfo.Methods = append(typeInfo.Methods, name.Name)
			}
		}
	}

	a.structure.Types[fmt.Sprintf("%s.%s", pkg, typ.Name.Name)] = typeInfo
}

// GenerateDocumentation generates documentation from the analyzed code
func (a *Analyzer) GenerateDocumentation() string {
	var doc strings.Builder

	// Project Overview
	doc.WriteString("# Code Structure Documentation\n\n")

	// Packages
	packages := make(map[string]bool)
	for _, file := range a.structure.Files {
		packages[file.Package] = true
	}
	doc.WriteString("## Packages\n\n")
	for pkg := range packages {
		doc.WriteString(fmt.Sprintf("- %s\n", pkg))
	}
	doc.WriteString("\n")

	// Files
	doc.WriteString("## Files\n\n")
	for path, file := range a.structure.Files {
		doc.WriteString(fmt.Sprintf("### %s\n", path))
		doc.WriteString(fmt.Sprintf("- Package: %s\n", file.Package))
		if len(file.Imports) > 0 {
			doc.WriteString("- Imports:\n")
			for _, imp := range file.Imports {
				doc.WriteString(fmt.Sprintf("  - %s\n", imp))
			}
		}
		doc.WriteString("\n")
	}

	// Types
	doc.WriteString("## Types\n\n")
	for _, typ := range a.structure.Types {
		doc.WriteString(fmt.Sprintf("### %s.%s\n", typ.Package, typ.Name))
		if typ.Doc != "" {
			doc.WriteString(fmt.Sprintf("Documentation:\n```\n%s```\n", typ.Doc))
		}
		if len(typ.Fields) > 0 {
			doc.WriteString("Fields:\n")
			for _, field := range typ.Fields {
				doc.WriteString(fmt.Sprintf("- %s\n", field))
			}
		}
		if len(typ.Methods) > 0 {
			doc.WriteString("Methods:\n")
			for _, method := range typ.Methods {
				doc.WriteString(fmt.Sprintf("- %s\n", method))
			}
		}
		doc.WriteString("\n")
	}

	// Functions
	doc.WriteString("## Functions\n\n")
	for _, fn := range a.structure.Functions {
		doc.WriteString(fmt.Sprintf("### %s.%s\n", fn.Package, fn.Name))
		if fn.Doc != "" {
			doc.WriteString(fmt.Sprintf("Documentation:\n```\n%s```\n", fn.Doc))
		}
		doc.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n", fn.File, fn.StartLine, fn.EndLine))
		if len(fn.Calls) > 0 {
			doc.WriteString("Calls:\n")
			for _, call := range fn.Calls {
				doc.WriteString(fmt.Sprintf("- %s\n", call))
			}
		}
		doc.WriteString("\n")
	}

	return doc.String()
}

// GetStructure returns the analyzed code structure
func (a *Analyzer) GetStructure() *CodeStructure {
	return a.structure
}
