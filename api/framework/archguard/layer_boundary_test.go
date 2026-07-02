package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/tfnick/go-svelte-starter"

type layerImportRule struct {
	dir       string
	forbidden []string
}

func TestCoreLayerImportsStayWithinArchitecture(t *testing.T) {
	rules := []layerImportRule{
		{
			dir: "api/models",
			forbidden: []string{
				modulePath + "/api/routes",
				modulePath + "/api/usecase",
				modulePath + "/api/framework/http",
			},
		},
		{
			dir: "api/usecase",
			forbidden: []string{
				modulePath + "/api/db",
				modulePath + "/api/providers",
				modulePath + "/api/routes",
				modulePath + "/api/framework/http",
			},
		},
		{
			dir: "api/routes",
			forbidden: []string{
				modulePath + "/api/db",
				modulePath + "/api/providers",
				modulePath + "/api/models",
			},
		},
		{
			dir: "api/providers",
			forbidden: []string{
				modulePath + "/api/db",
				modulePath + "/api/models",
				modulePath + "/api/routes",
				modulePath + "/api/framework/http",
			},
		},
	}

	var violations []string
	for _, rule := range rules {
		violations = append(violations, collectForbiddenImports(t, rule)...)
	}

	if len(violations) > 0 {
		t.Fatalf("core layer imports violate routes -> usecase -> models boundaries:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCoreLayerDirectoriesDoNotOwnFrameworkFiles(t *testing.T) {
	rules := map[string][]string{
		"api/models": {
			"helpers.go",
			"responses.go",
			"*_boundary_test.go",
			"*_guard_test.go",
		},
		"api/routes": {
			"helpers.go",
			"responses.go",
			"internal_errors.go",
			"*_boundary_test.go",
			"*_guard_test.go",
		},
		"api/usecase": {
			"helpers.go",
			"responses.go",
			"*_boundary_test.go",
			"*_guard_test.go",
		},
	}

	root := repoRoot(t)
	var violations []string
	for dir, forbiddenPatterns := range rules {
		files, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), "*.go"))
		if err != nil {
			t.Fatalf("list %s files: %v", dir, err)
		}
		for _, filePath := range files {
			name := filepath.Base(filePath)
			if !matchesAnyPattern(name, forbiddenPatterns) {
				continue
			}
			violations = append(violations, fmt.Sprintf("%s belongs under api/framework/<capability>, not %s", mustRel(t, root, filePath), dir))
		}
	}

	if len(violations) > 0 {
		t.Fatalf("core layer directories must not own framework/helper/guard files:\n%s", strings.Join(violations, "\n"))
	}
}

func TestInternalRoutesUseResponseEnvelopeHelpers(t *testing.T) {
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "api", "routes", "*.go"))
	if err != nil {
		t.Fatalf("list route files: %v", err)
	}

	fset := token.NewFileSet()
	var violations []string
	for _, filePath := range files {
		name := filepath.Base(filePath)
		if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "open_api_") {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if selector.Sel.Name != "JSON" {
				return true
			}

			if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "c" {
				pos := fset.Position(call.Pos())
				rel := mustRel(t, root, pos.Filename)
				violations = append(violations, fmt.Sprintf("%s:%d calls c.%s directly; use httpresponse envelope helpers", rel, pos.Line, selector.Sel.Name))
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Fatalf("internal routes must use response envelope helpers:\n%s", strings.Join(violations, "\n"))
	}
}

func TestDomainEventsStayQueueBacked(t *testing.T) {
	root := repoRoot(t)
	files := collectGoFiles(t, filepath.Join(root, "api"))
	var violations []string

	fset := token.NewFileSet()
	for _, filePath := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		rel := mustRel(t, root, filePath)
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if path == "github.com/asaskevich/EventBus" {
				violations = append(violations, fmt.Sprintf("%s imports retired raw EventBus dependency", rel))
			}
			if path == modulePath+"/api/framework/events" && strings.HasPrefix(rel, "api/models/") {
				violations = append(violations, fmt.Sprintf("%s imports framework events from models", rel))
			}
			if (path == "maragu.dev/goqite" || strings.HasPrefix(path, "maragu.dev/goqite/")) && !strings.HasPrefix(rel, "api/framework/queue/") {
				violations = append(violations, fmt.Sprintf("%s imports raw goqite dependency", rel))
			}
		}
	}

	if _, err := os.Stat(filepath.Join(root, "api", "framework", "outbox")); err == nil {
		violations = append(violations, "api/framework/outbox must not be created; use api/framework/events with api/framework/queue")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat api/framework/outbox: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("domain event architecture boundary violated:\n%s", strings.Join(violations, "\n"))
	}
}

func TestQueueDoesNotDependOnUsecaseFramework(t *testing.T) {
	root := repoRoot(t)
	files := collectGoFiles(t, filepath.Join(root, "api", "framework", "queue"))
	var violations []string

	fset := token.NewFileSet()
	for _, filePath := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}
		rel := mustRel(t, root, filePath)
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if path == modulePath+"/api/framework/usecase" || strings.HasPrefix(path, modulePath+"/api/framework/usecase/") {
				violations = append(violations, fmt.Sprintf("%s imports framework usecase; queue must depend on api/db transaction hooks instead", rel))
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("queue architecture boundary violated:\n%s", strings.Join(violations, "\n"))
	}
}

func TestBusinessRealtimePublishingStaysBehindNotificationBoundary(t *testing.T) {
	root := repoRoot(t)
	files := collectGoFiles(t, filepath.Join(root, "api"))
	var violations []string

	fset := token.NewFileSet()
	for _, filePath := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		rel := mustRel(t, root, filePath)
		if isAllowedRealtimeImport(rel) {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if path == modulePath+"/api/framework/realtime" {
				violations = append(violations, fmt.Sprintf("%s imports framework realtime directly; route through notification usecase/helper", rel))
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("business realtime publishing boundary violated:\n%s", strings.Join(violations, "\n"))
	}
}

func isAllowedRealtimeImport(rel string) bool {
	return strings.HasPrefix(rel, "api/framework/realtime/") ||
		rel == "api/framework/realtime/realtime.go" ||
		rel == "api/usecase/notification.go" ||
		rel == "api/routes/points.go"
}

func TestDurableEventTablesStayProjectOwned(t *testing.T) {
	root := repoRoot(t)
	migrations := collectFilesByExt(t, filepath.Join(root, "api", "db", "migrations", "sqlite", "app"), ".sql")
	var violations []string
	for _, filePath := range migrations {
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read %s: %v", filePath, err)
		}
		rel := mustRel(t, root, filePath)
		text := strings.ToLower(string(content))
		if strings.Contains(text, "domain_event_executions") {
			violations = append(violations, fmt.Sprintf("%s creates retired domain_event_executions table", rel))
		}
		if strings.Contains(text, "create table if not exists goqite") && !strings.HasSuffix(rel, "007_add_goqite.sql") {
			violations = append(violations, fmt.Sprintf("%s creates goqite-owned table outside goqite migration", rel))
		}
		if strings.HasSuffix(rel, "007_add_goqite.sql") {
			for _, projectTable := range []string{"scheduled_tasks", "scheduled_task_executions", "domain_events", "domain_event_deliveries"} {
				if strings.Contains(text, projectTable) {
					violations = append(violations, fmt.Sprintf("%s mixes project-owned table %s into goqite-owned migration", rel, projectTable))
				}
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("durable event/table ownership violated:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCookieSessionAuthIsRetired(t *testing.T) {
	root := repoRoot(t)
	files := collectGoFiles(t, filepath.Join(root, "api"))
	var violations []string

	for _, filePath := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read %s: %v", filePath, err)
		}
		text := string(content)
		for _, forbidden := range []string{
			"SessionCookieName",
			"SetSessionCookie",
			"CreateSession(",
			"GetSessionByID(",
			"DeleteSession(",
			"DeleteUserSessions(",
			"CleanExpiredSessions(",
		} {
			if strings.Contains(text, forbidden) {
				violations = append(violations, fmt.Sprintf("%s contains retired cookie session auth symbol %q", mustRel(t, root, filePath), forbidden))
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("cookie/session auth is retired; use JWT middleware instead:\n%s", strings.Join(violations, "\n"))
	}
}

func collectForbiddenImports(t *testing.T, rule layerImportRule) []string {
	t.Helper()

	root := repoRoot(t)
	files := collectGoFiles(t, filepath.Join(root, filepath.FromSlash(rule.dir)))

	fset := token.NewFileSet()
	var violations []string
	for _, filePath := range files {
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}

		parsed, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			for _, forbidden := range rule.forbidden {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					rel := mustRel(t, root, filePath)
					violations = append(violations, fmt.Sprintf("%s imports %s", rel, path))
				}
			}
		}
	}
	return violations
}

func collectGoFiles(t *testing.T, dir string) []string {
	t.Helper()

	return collectFilesByExt(t, dir, ".go")
}

func collectFilesByExt(t *testing.T, dir string, ext string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return files
}

func matchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if !strings.ContainsAny(pattern, "*?[") {
			if name == pattern {
				return true
			}
			continue
		}
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func mustRel(t *testing.T, base string, target string) string {
	t.Helper()

	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("rel path for %s: %v", target, err)
	}
	return filepath.ToSlash(rel)
}
