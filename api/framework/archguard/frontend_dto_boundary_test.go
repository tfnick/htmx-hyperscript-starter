package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

type modelVarInfo struct {
	source string
}

func TestInternalRoutesDoNotReturnModelsDirectly(t *testing.T) {
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "api", "routes", "*.go"))
	if err != nil {
		t.Fatalf("list route files: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string
	for _, filePath := range files {
		if skipDTOBoundaryFile(filePath) {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			modelVars := map[string]modelVarInfo{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.AssignStmt:
					recordAssignedModelVars(modelVars, n.Lhs, n.Rhs)
				case *ast.ValueSpec:
					recordDeclaredModelVars(modelVars, n)
				case *ast.CallExpr:
					if !isJSONCall(n) || len(n.Args) < 2 {
						return true
					}

					payload := n.Args[len(n.Args)-1]
					if reason := directModelPayloadReason(payload, modelVars); reason != "" {
						pos := fset.Position(payload.Pos())
						rel := mustRel(t, root, pos.Filename)
						violations = append(violations, fmt.Sprintf("%s:%d: %s", rel, pos.Line, reason))
					}
				}
				return true
			})
		}
	}

	if len(violations) > 0 {
		t.Fatalf("internal route resource responses must return DTOs, not models:\n%s", strings.Join(violations, "\n"))
	}
}

func skipDTOBoundaryFile(filePath string) bool {
	name := filepath.Base(filePath)
	return strings.HasSuffix(name, "_test.go") ||
		strings.HasPrefix(name, "open_api_")
}

func recordAssignedModelVars(modelVars map[string]modelVarInfo, lhs []ast.Expr, rhs []ast.Expr) {
	if len(rhs) == 1 {
		source := modelSourceDescription(rhs[0])
		if source == "" {
			return
		}
		for _, expr := range lhs {
			recordModelVar(modelVars, expr, source)
		}
		return
	}

	for i, expr := range lhs {
		if i >= len(rhs) {
			continue
		}
		source := modelSourceDescription(rhs[i])
		if source == "" {
			continue
		}
		recordModelVar(modelVars, expr, source)
	}
}

func recordDeclaredModelVars(modelVars map[string]modelVarInfo, spec *ast.ValueSpec) {
	source := ""
	if isModelsType(spec.Type) {
		source = "models type declaration"
	}
	for _, value := range spec.Values {
		if valueSource := modelSourceDescription(value); valueSource != "" {
			source = valueSource
		}
	}
	if source == "" {
		return
	}

	for _, name := range spec.Names {
		recordModelVar(modelVars, name, source)
	}
}

func recordModelVar(modelVars map[string]modelVarInfo, expr ast.Expr, source string) {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "_" || ident.Name == "err" {
		return
	}
	modelVars[ident.Name] = modelVarInfo{source: source}
}

func modelSourceDescription(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if name := modelsCallName(e.Fun); name != "" {
			return "models." + name + "()"
		}
	case *ast.CompositeLit:
		if isModelsType(e.Type) {
			return "models composite literal"
		}
	case *ast.UnaryExpr:
		return modelSourceDescription(e.X)
	case *ast.ParenExpr:
		return modelSourceDescription(e.X)
	}
	return ""
}

func directModelPayloadReason(expr ast.Expr, modelVars map[string]modelVarInfo) string {
	switch e := expr.(type) {
	case *ast.Ident:
		if info, ok := modelVars[e.Name]; ok {
			return fmt.Sprintf("direct model variable %q from %s", e.Name, info.source)
		}
	case *ast.CallExpr:
		if name := modelsCallName(e.Fun); name != "" {
			return "direct models." + name + "() call"
		}
	case *ast.CompositeLit:
		if isModelsType(e.Type) {
			return "direct models composite literal"
		}
		for _, elt := range e.Elts {
			value := elt
			if keyValue, ok := elt.(*ast.KeyValueExpr); ok {
				value = keyValue.Value
			}
			if reason := directModelPayloadReason(value, modelVars); reason != "" {
				return reason
			}
		}
	case *ast.UnaryExpr:
		return directModelPayloadReason(e.X, modelVars)
	case *ast.ParenExpr:
		return directModelPayloadReason(e.X, modelVars)
	}
	return ""
}

func isJSONCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "JSON"
}

func modelsCallName(expr ast.Expr) string {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident.Name != "models" {
		return ""
	}

	return selector.Sel.Name
}

func isModelsType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		ident, ok := e.X.(*ast.Ident)
		return ok && ident.Name == "models"
	case *ast.StarExpr:
		return isModelsType(e.X)
	case *ast.ArrayType:
		return isModelsType(e.Elt)
	}
	return false
}
