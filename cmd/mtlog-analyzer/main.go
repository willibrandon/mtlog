package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"strings"

	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
)

// StdinRequest represents the input format when analyzing from stdin
type StdinRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	GoModule string `json:"go_module"` // Path to go.mod directory
}

// StdinDiagnostic represents a diagnostic in the output format
type StdinDiagnostic struct {
	Filename  string `json:"filename"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
	DiagnosticID string `json:"diagnostic_id,omitempty"`
}

func main() {
	// Check for -stdin flag before singlechecker processes args
	stdinMode := false
	fmt.Fprintf(os.Stderr, "Args: %v\n", os.Args)
	for _, arg := range os.Args[1:] {
		if arg == "-stdin" {
			stdinMode = true
			break
		}
	}

	if stdinMode {
		fmt.Fprintf(os.Stderr, "Running in stdin mode\n")
		runStdinMode()
	} else {
		// Regular go vet mode
		singlechecker.Main(analyzer.Analyzer)
	}
}

func runStdinMode() {
	// Read JSON request from stdin
	var req StdinRequest
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode stdin: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Stdin mode: analyzing %s (%d chars)\n", req.Filename, len(req.Content))
	// Debug: show first few lines of content
	lines := strings.Split(req.Content, "\n")
	for i, line := range lines {
		if i >= 5 {
			break
		}
		fmt.Fprintf(os.Stderr, "  Line %d: %s\n", i+1, line)
	}

	// Analyze the provided content
	diagnostics, err := analyzeContent(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Analysis failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d diagnostics\n", len(diagnostics))

	// Output diagnostics as JSON (ensure empty array instead of null)
	if diagnostics == nil {
		diagnostics = []StdinDiagnostic{}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(diagnostics); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode output: %v\n", err)
		os.Exit(1)
	}
}

func analyzeContent(req StdinRequest) ([]StdinDiagnostic, error) {
	// First try to detect if we're in a module context
	hasModule := req.GoModule != ""
	
	// Use packages.Load with an overlay for the stdin content
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | 
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir: req.GoModule,
		Env: os.Environ(),
		Overlay: map[string][]byte{
			req.Filename: []byte(req.Content),
		},
	}
	
	// If no module directory provided, try to work with current directory
	if cfg.Dir == "" {
		cfg.Dir = "."
	}
	
	// Load the package containing our file
	// Try to determine the package pattern from the file
	pattern := "."
	if !hasModule {
		// For non-module mode, use file= pattern
		pattern = "file=" + req.Filename
	}
	
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		// If packages.Load fails completely, we could fall back to syntax-only analysis
		// For now, we'll still return the error but log it
		fmt.Fprintf(os.Stderr, "Warning: packages.Load failed: %v\n", err)
		return nil, fmt.Errorf("failed to load package: %w", err)
	}
	
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found")
	}
	
	// Handle multiple packages if necessary
	var pkg *packages.Package
	for _, p := range pkgs {
		// Find the package that contains our file
		for _, f := range p.GoFiles {
			if f == req.Filename {
				pkg = p
				break
			}
		}
		if pkg != nil {
			break
		}
	}
	
	// If we didn't find a package with our file, just use the first one
	if pkg == nil && len(pkgs) > 0 {
		pkg = pkgs[0]
	}
	
	if pkg == nil {
		return nil, fmt.Errorf("no package found containing %s", req.Filename)
	}
	
	if len(pkg.Errors) > 0 {
		for _, err := range pkg.Errors {
			fmt.Fprintf(os.Stderr, "Package error: %v\n", err)
		}
		// Don't fail on package errors - we can still do syntax-level analysis
	}
	
	// Find our file in the package
	var targetFile *ast.File
	for _, f := range pkg.Syntax {
		if pkg.Fset.Position(f.Pos()).Filename == req.Filename {
			targetFile = f
			break
		}
	}
	
	if targetFile == nil {
		return nil, fmt.Errorf("could not find target file in package")
	}

	// Create an analysis pass
	pass := &analysis.Pass{
		Analyzer:  analyzer.Analyzer,
		Fset:      pkg.Fset,
		Files:     []*ast.File{targetFile},
		Pkg:       pkg.Types,
		TypesInfo: pkg.TypesInfo,
		Report: func(diag analysis.Diagnostic) {
			// Handled below
		},
	}

	// Collect diagnostics
	var diagnostics []StdinDiagnostic
	pass.Report = func(diag analysis.Diagnostic) {
		pos := pkg.Fset.Position(diag.Pos)
		
		// Extract diagnostic ID from message if present
		diagID := ""
		message := diag.Message
		if strings.HasPrefix(message, "[MTLOG") {
			if idx := strings.Index(message, "]"); idx > 0 {
				diagID = message[1:idx]
				message = strings.TrimSpace(message[idx+1:])
			}
		}

		// Determine severity
		severity := "warning"
		if strings.Contains(strings.ToLower(message), "error") {
			severity = "error"
		} else if strings.HasPrefix(message, "suggestion:") {
			severity = "suggestion"
		}

		diagnostics = append(diagnostics, StdinDiagnostic{
			Filename:     req.Filename,
			Line:         pos.Line,
			Column:       pos.Column,
			Message:      diag.Message,
			Severity:     severity,
			DiagnosticID: diagID,
		})
	}

	// Initialize required analyzer state
	pass.ResultOf = make(map[*analysis.Analyzer]interface{})
	
	// Also add inspect.Analyzer result if needed
	if len(analyzer.Analyzer.Requires) > 0 {
		// The analyzer requires inspect.Analyzer
		inspect := inspector.New([]*ast.File{targetFile})
		pass.ResultOf[analyzer.Analyzer.Requires[0]] = inspect
		fmt.Fprintf(os.Stderr, "Added inspector to ResultOf\n")
	}

	fmt.Fprintf(os.Stderr, "Running analyzer.Run...\n")
	// Run the analyzer
	result, err := analyzer.Analyzer.Run(pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Analyzer.Run error: %v\n", err)
		return nil, fmt.Errorf("analyzer error: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Analyzer.Run completed, result type: %T\n", result)

	_ = result // Result is not used for diagnostics

	fmt.Fprintf(os.Stderr, "Returning %d diagnostics\n", len(diagnostics))
	return diagnostics, nil
}

