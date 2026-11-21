//go:build gtk

package desktop

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"testing"
)

func TestFrenchCatalog(t *testing.T) {
	format := regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?[a-zA-Z%]`)
	for key, value := range french {
		if value == "" || !reflect.DeepEqual(format.FindAllString(key, -1), format.FindAllString(value, -1)) {
			t.Errorf("invalid translation or placeholders: %q", key)
		}
	}
	for _, path := range []string{"desktop.go", "studio.go", "project.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch f := call.Fun.(type) {
			case *ast.Ident:
				name = f.Name
			case *ast.SelectorExpr:
				name = f.Sel.Name
			}
			index := 0
			switch name {
			case "t", "bind", "label", "button", "menu":
			case "translate":
				index = 1
			case "item", "tool":
				index = 2
			default:
				return true
			}
			if len(call.Args) <= index {
				return true
			}
			if literal, ok := call.Args[index].(*ast.BasicLit); ok && literal.Kind == token.STRING {
				key, _ := strconv.Unquote(literal.Value)
				if _, ok := french[key]; !ok {
					t.Errorf("missing French label in %s: %q", path, key)
				}
			}
			return true
		})
	}
	if groupName("fr", 3) != "Groupe 3" || newProject("fr").Name != "Projet sans titre" {
		t.Fatal("French project defaults")
	}
	if got := errorText("fr", errors.New("empty track path")); got != "Chemin de piste vide" {
		t.Fatal(got)
	}
}
