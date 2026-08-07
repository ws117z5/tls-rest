package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"tls-rest/go/constants"

	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"text/template"
)

type Function struct {
	Name string
}

var sql = false
var routes = true

func main() {

	flag.BoolVar(&sql, "sql", false, "init sql structure")
	flag.BoolVar(&routes, "routes", true, "init routes")
	flag.Parse()

	if sql {
		initSQLFiles()
	}

	if routes {
		initRoutes()
	}
}

func initSQLFiles() {
	entries, err := os.ReadDir(constants.SQLPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(constants.SQLPath, entry.Name())
			cmd := exec.Command("psql", "-d", "postgres", "-f", fullPath) // Example command: `ls -la`

			// Set the command's output to the terminal
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Run the command
			if err := cmd.Run(); err != nil {
				log.Fatal(err)
			}
		}
	}

	entries, err = os.ReadDir(constants.SQLBackupPath)
	if err != nil {
		log.Fatal(err)
	}

	//if we have backups, we apply them to the database
	if len(entries) > 0 {

		log.Println("Done running structure scripts, now apply backups")
		// Convert entries to a slice of os.FileInfo to get modification time
		fileInfos := make([]os.FileInfo, 0, len(entries))

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				log.Fatal(err)
			}
			fileInfos = append(fileInfos, info)
		}

		// Sort by creation time (oldest to newest)
		sort.Slice(fileInfos, func(i, j int) bool {
			return fileInfos[i].ModTime().After(fileInfos[j].ModTime())
		})

		log.Println(fileInfos[0])
	}

	// for _, entry := range entries {
	// 	if !entry.IsDir() {
	// 		fullPath := filepath.Join(constants.SQLBackupPath, entry.Name())
	// 		cmd := exec.Command("psql", "-d", "postgres", "-f", fullPath) // Example command: `ls -la`

	// 		// Set the command's output to the terminal
	// 		cmd.Stdout = os.Stdout
	// 		cmd.Stderr = os.Stderr

	// 		// Run the command
	// 		if err := cmd.Run(); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 	}
	// }
}

func initRoutes() {
	// Directories to scan
	//todo set right directories + do db read
	dirs := []string{"./handlers", "./routes"}

	var functions []Function
	fset := token.NewFileSet()

	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}

			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Type == nil || fn.Type.Params == nil || fn.Name == nil {
					continue
				}

				params := fn.Type.Params.List
				if len(params) != 2 {
					continue
				}

				// Check: (w http.ResponseWriter, r *http.Request)
				if isResponseWriter(params[0]) && isRequestPointer(params[1]) {
					functions = append(functions, Function{Name: fn.Name.Name})
				}
			}

			return nil
		})
	}

	outputFile, err := os.Create("autogen_funcmap.go")
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	tmplBytes, err := os.ReadFile("init/routes.tpl")
	if err != nil {
		log.Fatal("Failed to read template file:", err)
	}

	tmpl := template.Must(template.New("funcmap").Parse(string(tmplBytes)))
	tmpl.Execute(outputFile, functions)
}

func isResponseWriter(field *ast.Field) bool {
	sel, ok := field.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "http" && sel.Sel.Name == "ResponseWriter"
}

func isRequestPointer(field *ast.Field) bool {
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "http" && sel.Sel.Name == "Request"
}
