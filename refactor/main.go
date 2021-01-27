package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"os"
	"path/filepath"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/goast"
	"github.com/dave/dst/decorator/resolver/guess"
	"golang.org/x/tools/go/packages"
)

func main() {
	// find the absolute path to the example.
	dir, err := filepath.Abs("/home/cell/code/cloud-on-k8s/")
	if err != nil {
		exitOnErr(err)
	}

	// load the packages
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode:  packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles | packages.NeedSyntax | packages.NeedName | packages.NeedImports | packages.NeedDeps,
		Dir:   dir,
		Fset:  fset,
		Logf:  log.Printf,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, "./...")
	exitOnErr(err)

	// create a new decorator with support for resolving imports
	//d := decorator.NewDecoratorWithImports(fset, "main", goast.New())
	d := decorator.NewDecoratorWithImports(fset, "main", goast.WithResolver(guess.WithMap(map[string]string{"github.com/elastic/cloud-on-k8s/pkg/utils/log": "ulog"})))

	// rewrite sources
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			rewrite(d, pkg, f, iface)
		}
	}
}

func rewrite(d *decorator.Decorator, pkg *packages.Package, f *ast.File) {
	updated := false

	df, err := d.DecorateFile(f)
	exitOnErr(err)

	// traverse the AST.
	dst.Inspect(df, func(node dst.Node) bool {
		if node == nil {
			return false
		}

		ve, ok := node.(*dst.ValueSpec)
		if !ok || len(ve.Values) != 1 {
			return true
		}

		ce, ok := ve.Values[0].(*dst.CallExpr)
		if !ok {
			return true
		}

		se, ok := ce.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		//dst.Fprint(os.Stdout, se, nil)

		ident, ok := se.X.(*dst.Ident)
		if !ok {
			return true
		}

		if ident.Path == "sigs.k8s.io/controller-runtime/pkg/log" && ident.Name == "Log" {
			log.Printf("Match found at %s", d.Fset.Position(d.Ast.Nodes[se].Pos()))
			newIdent := dst.NewIdent("Log")
			newIdent.Path = "github.com/elastic/cloud-on-k8s/pkg/utils/log"
			se.X = newIdent
			updated = true
		}

		return true
	})

	if updated {
		p := d.Fset.Position(f.Pos())
		writeFile(p.Filename, df)
	}
}

func writeFile(fileName string, df *dst.File) {
	log.Printf("Writing changes to %s", fileName)

	restorer := decorator.NewRestorerWithImports("main", guess.WithMap(map[string]string{"github.com/elastic/cloud-on-k8s/pkg/utils/log": "ulog"}))

	out, err := os.Create(fileName)
	if err != nil {
		exitOnErr(fmt.Errorf("failed to open %s for writing", fileName, err))
	}

	defer out.Close()

	fr := restorer.FileRestorer()
	fr.Alias["github.com/elastic/cloud-on-k8s/pkg/utils/log"] = "ulog"

	exitOnErr(fr.Fprint(out, df))
}

func exitOnErr(err error) {
	if err != nil {
		log.Fatalf("ERROR: %v\n", err)
	}
}
