package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

// checkErrorLoggingWithConfig checks for proper error logging patterns
func checkErrorLoggingWithConfig(pass *analysis.Pass, call *ast.CallExpr, config *Config) {
	methodName := getMethodName(call)
	if !isErrorMethod(methodName) {
		return
	}
	
	// Check if an error is being logged
	hasError := false
	if pass.TypesInfo != nil {
		for i := 1; i < len(call.Args); i++ {
			argType := pass.TypesInfo.TypeOf(call.Args[i])
			if argType != nil && isErrorType(argType) {
				hasError = true
				break
			}
		}
	}
	
	if !hasError {
		// Check if diagnostic is suppressed
		if config.SuppressedDiagnostics[DiagIDErrorLogging] {
			return
		}
		
		// Create diagnostic with suggested fix
		diagnostic := analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: fmt.Sprintf("[%s] %s: Error level log without error value, consider including the error or using Warning level", DiagIDErrorLogging, SeveritySuggestion),
		}
		
		// Find error variable in scope or use nil
		errorVar := findErrorVariableInScope(pass, call)
		errorParam := errorVar
		if errorParam == "" {
			errorParam = "nil"
		}
		
		// Add suggested fix to add error parameter
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Add error parameter",
			TextEdits: []analysis.TextEdit{{
				Pos:     call.End() - 1, // Just before the closing paren
				End:     call.End() - 1,
				NewText: []byte(", " + errorParam),
			}},
		}}
		
		// If using nil, add TODO comment
		if errorParam == "nil" {
			addErrorTodoComment(pass, call, &diagnostic)
		}
		
		pass.Report(diagnostic)
	}
}

// addErrorTodoComment adds a TODO comment for missing error values
func addErrorTodoComment(pass *analysis.Pass, call *ast.CallExpr, diagnostic *analysis.Diagnostic) {
	pos := pass.Fset.Position(call.Pos())
	
	// Find the file containing this call
	var targetFile *ast.File
	for _, f := range pass.Files {
		if f.Pos() <= call.Pos() && call.End() <= f.End() {
			targetFile = f
			break
		}
	}
	
	if targetFile == nil {
		return
	}
	
	// Check for comments on the same line
	hasComment := false
	for _, cg := range targetFile.Comments {
		for _, c := range cg.List {
			cPos := pass.Fset.Position(c.Pos())
			if cPos.Line == pos.Line {
				hasComment = true
				break
			}
		}
		if hasComment {
			break
		}
	}
	
	// Calculate indent
	parent := findParentStmt(targetFile, call)
	indent := ""
	if parent != nil {
		parentPos := pass.Fset.Position(parent.Pos())
		if parentPos.Line == pos.Line && parentPos.Column > 1 {
			for i := 1; i < parentPos.Column; i++ {
				indent += " "
			}
		}
	}
	
	// Find line end
	lineEnd := call.End()
	ast.Inspect(targetFile, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		// Skip comments
		if _, isComment := n.(*ast.Comment); isComment {
			return true
		}
		if _, isCommentGroup := n.(*ast.CommentGroup); isCommentGroup {
			return true
		}
		
		nEnd := pass.Fset.Position(n.End())
		if nEnd.Line == pos.Line && n.End() > lineEnd {
			lineEnd = n.End()
		}
		
		return pass.Fset.Position(n.Pos()).Line <= pos.Line
	})
	
	// Add the text edit
	if hasComment {
		// Find comment end
		var commentEnd token.Pos = lineEnd
		for _, cg := range targetFile.Comments {
			for _, c := range cg.List {
				cPos := pass.Fset.Position(c.Pos())
				if cPos.Line == pos.Line && c.End() > commentEnd {
					commentEnd = c.End()
				}
			}
		}
		
		diagnostic.SuggestedFixes[0].TextEdits = append(diagnostic.SuggestedFixes[0].TextEdits, analysis.TextEdit{
			Pos:     commentEnd,
			End:     commentEnd,
			NewText: []byte("\n" + indent + "// TODO: replace nil with actual error"),
		})
	} else {
		diagnostic.SuggestedFixes[0].TextEdits = append(diagnostic.SuggestedFixes[0].TextEdits, analysis.TextEdit{
			Pos:     lineEnd,
			End:     lineEnd,
			NewText: []byte(" // TODO: replace nil with actual error"),
		})
	}
}

// findParentStmt finds the parent statement node for a given node
func findParentStmt(file *ast.File, target ast.Node) ast.Stmt {
	var parent ast.Stmt
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt, ok := n.(ast.Stmt); ok {
			// Check if this statement contains our target
			if stmt.Pos() <= target.Pos() && target.End() <= stmt.End() {
				parent = stmt
			}
		}
		return true
	})
	return parent
}

// findErrorVariableInScope finds an error variable in scope
func findErrorVariableInScope(pass *analysis.Pass, call *ast.CallExpr) string {
	if pass.Fset == nil || len(pass.Files) == 0 {
		return ""
	}
	
	// Find the containing function
	var funcType *ast.FuncType
	var funcBody *ast.BlockStmt
	
	// Walk up the AST to find the closest function
	path, _ := astutil.PathEnclosingInterval(pass.Files[0], call.Pos(), call.End())
	for _, node := range path {
		switch n := node.(type) {
		case *ast.FuncDecl:
			funcType = n.Type
			funcBody = n.Body
		case *ast.FuncLit:
			funcType = n.Type
			funcBody = n.Body
		}
		if funcType != nil {
			break
		}
	}
	
	if funcType == nil || funcBody == nil {
		return ""
	}
	
	// Check function parameters first
	if funcType.Params != nil && pass.TypesInfo != nil {
		for _, field := range funcType.Params.List {
			typ := pass.TypesInfo.TypeOf(field.Type)
			if typ != nil && isErrorType(typ) {
				for _, name := range field.Names {
					return name.Name
				}
			}
		}
	}
	
	// Check named return values
	if funcType.Results != nil && pass.TypesInfo != nil {
		for _, field := range funcType.Results.List {
			typ := pass.TypesInfo.TypeOf(field.Type)
			if typ != nil && isErrorType(typ) && len(field.Names) > 0 {
				for _, name := range field.Names {
					return name.Name
				}
			}
		}
	}
	
	// Look for error variables in the function body
	callPos := pass.Fset.Position(call.Pos())
	var bestVar string
	var bestLine int = -1
	
	ast.Inspect(funcBody, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
			nodePos := pass.Fset.Position(assign.Pos())
			if nodePos.Line < callPos.Line {
				for _, lhs := range assign.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						if isLikelyErrorVariable(ident.Name) {
							// Check if we're in an if block
							inIf := false
							ast.Inspect(funcBody, func(n2 ast.Node) bool {
								if ifStmt, ok := n2.(*ast.IfStmt); ok {
									if call.Pos() >= ifStmt.Pos() && call.End() <= ifStmt.End() {
										inIf = true
										return false
									}
								}
								return true
							})
							
							// Only use error vars if we're in an if block and it's recent
							if inIf && (callPos.Line - nodePos.Line) <= 5 {
								if nodePos.Line > bestLine {
									bestVar = ident.Name
									bestLine = nodePos.Line
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	
	return bestVar
}

// isLikelyErrorVariable checks if a variable name looks like an error
func isLikelyErrorVariable(name string) bool {
	errorNames := []string{"err", "error", "e", "errs", "errors"}
	nameLower := strings.ToLower(name)
	
	for _, errorName := range errorNames {
		if nameLower == errorName {
			return true
		}
	}
	
	return strings.HasSuffix(name, "Err") || 
	       strings.HasSuffix(name, "Error") ||
	       strings.HasPrefix(nameLower, "err") ||
	       strings.HasPrefix(nameLower, "error")
}