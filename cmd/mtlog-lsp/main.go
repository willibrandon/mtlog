// mtlog-lsp provides a Language Server Protocol implementation for mtlog-analyzer.
// This version bundles the analyzer directly, eliminating the need for a separate binary.
// It wraps the mtlog-analyzer static analysis tool, converting its diagnostics
// and suggested fixes to LSP code actions, while handling the JSON-RPC communication
// protocol required by LSP.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	analyzer "github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
)

const (
	// editContextLength is the number of characters to show before and after an edit location for debugging
	editContextLength = 20
)

// WorkspaceConfiguration holds configuration options received from the client.
type WorkspaceConfiguration struct {
	Mtlog struct {
		SuppressedCodes        []string          `json:"suppressedCodes"`
		SeverityOverrides      map[string]string `json:"severityOverrides"`
		DisableAll            bool              `json:"disableAll"`
		CommonKeys            []string          `json:"commonKeys"`
		StrictMode            bool              `json:"strictMode"`
		IgnoreDynamicTemplates bool              `json:"ignoreDynamicTemplates"`
	} `json:"mtlog"`
	// Analyzer-specific configuration
	Analyzer struct {
		Strict                 bool     `json:"strict"`
		CommonKeys             []string `json:"commonKeys"`
		DisabledChecks         []string `json:"disabledChecks"`
		IgnoreDynamicTemplates bool     `json:"ignoreDynamicTemplates"`
		StrictLoggerTypes      bool     `json:"strictLoggerTypes"`
		DowngradeErrors        bool     `json:"downgradeErrors"`
		CheckReservedProps     bool     `json:"checkReservedProps"`
		ReservedProps          []string `json:"reservedProps"`
	} `json:"analyzerConfig"`
}

// Server manages the LSP server state and caches.
// It maintains separate caches for diagnostics and their associated code actions
// to efficiently handle LSP requests without re-running the analyzer unnecessarily.
type Server struct {
	rootPath     string
	logger       *log.Logger
	config       WorkspaceConfiguration // Workspace configuration from client
	// Store diagnostics and their fixes separately
	diagnosticsCache map[string][]Diagnostic // uri -> diagnostics
	fixesCache       map[string]map[string][]CodeAction // uri -> diagnostic key -> fixes
}

// CodeAction represents an LSP code action that can be applied to fix diagnostics.
// Code actions are cached and associated with specific diagnostics via a key.
type CodeAction struct {
	Title       string                 `json:"title"`
	Kind        string                 `json:"kind,omitempty"`
	Diagnostics []Diagnostic          `json:"diagnostics,omitempty"`
	Edit        map[string]interface{} `json:"edit,omitempty"`
}

// JSONRPCMessage represents a JSON-RPC 2.0 message used in LSP communication.
type JSONRPCMessage struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// InitializeParams contains the parameters sent by the client during initialization.
type InitializeParams struct {
	RootPath string `json:"rootPath,omitempty"`
	RootURI  string `json:"rootUri,omitempty"`
}

// Diagnostic represents an LSP diagnostic with position, severity, and message.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// Range represents a text range in a document using start and end positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document using zero-based line and character indices.
// The character index uses UTF-16 code units as per LSP specification.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func main() {
	// Set up logging to stderr (stdout is for LSP communication)
	logger := log.New(os.Stderr, "[mtlog-lsp] ", log.LstdFlags)
	
	logger.Printf("Starting bundled mtlog-lsp with integrated analyzer")
	
	// Create server
	server := &Server{
		logger:           logger,
		diagnosticsCache: make(map[string][]Diagnostic),
		fixesCache:       make(map[string]map[string][]CodeAction),
	}
	
	// Set up LSP communication
	reader := bufio.NewReader(os.Stdin)
	
	for {
		// Read LSP headers
		var contentLength int
		for {
			header, err := reader.ReadString('\n')
			if err != nil {
				logger.Printf("Error reading header: %v", err)
				os.Exit(1)
			}
			
			header = strings.TrimSpace(header)
			if header == "" {
				break // End of headers
			}
			
			if strings.HasPrefix(header, "Content-Length:") {
				contentLengthLine := strings.TrimSpace(header)
				_, _ = fmt.Sscanf(contentLengthLine, "Content-Length: %d", &contentLength)
			}
		}
		
		if contentLength == 0 {
			continue
		}
		
		// Read the JSON-RPC message
		msgBytes := make([]byte, contentLength)
		_, err := io.ReadFull(reader, msgBytes)
		if err != nil {
			logger.Printf("Error reading message: %v", err)
			continue
		}
		
		// Parse and handle the message
		var msg JSONRPCMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			logger.Printf("Error parsing message: %v", err)
			continue
		}
		
		logger.Printf("Received method: %s", msg.Method)
		
		// Handle different LSP methods
		switch msg.Method {
		case "initialize":
			server.handleInitialize(msg.ID, msg.Params)
		case "initialized":
			// No response needed
		case "textDocument/didOpen":
			server.handleDidOpen(msg.Params)
		case "textDocument/didSave":
			server.handleDidSave(msg.Params)
		case "textDocument/didChange":
			// We'll analyze on save, not on change
		case "textDocument/codeAction":
			server.handleCodeAction(msg.ID, msg.Params)
		case "workspace/didChangeConfiguration":
			// Configuration changes are handled through restart with new initializationOptions
			logger.Printf("Configuration changed - restart required")
		case "shutdown":
			// Clean up caches before shutdown
			server.cleanup()
			sendResponse(msg.ID, nil)
			os.Exit(0)
		case "exit":
			// Final cleanup on exit
			server.cleanup()
			os.Exit(0)
		}
	}
}

// cleanup clears all caches and performs shutdown tasks.
// It is called during both shutdown and exit to ensure proper resource cleanup.
func (s *Server) cleanup() {
	s.logger.Printf("Cleaning up caches")
	// Clear all caches
	s.diagnosticsCache = make(map[string][]Diagnostic)
	s.fixesCache = make(map[string]map[string][]CodeAction)
}

// sendResponse sends a JSON-RPC response with the given ID and result.
func sendResponse(id interface{}, result interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	
	data, _ := json.Marshal(response)
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(data), data)
}

// sendDiagnostics sends diagnostic notifications to the client for a specific file.
// It publishes diagnostics immediately when analysis completes.
func sendDiagnostics(uri string, diagnostics []Diagnostic) {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]interface{}{
			"uri":         uri,
			"diagnostics": diagnostics,
		},
	}
	
	data, _ := json.Marshal(notification)
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(data), data)
}

// handleInitialize processes the LSP initialize request and responds with server capabilities.
// It sets up the root path for analysis and advertises support for text synchronization and code actions.
func (s *Server) handleInitialize(id interface{}, params json.RawMessage) {
	
	var initParams InitializeParams
	_ = json.Unmarshal(params, &initParams)
	
	if initParams.RootPath != "" {
		s.rootPath = initParams.RootPath
	} else if initParams.RootURI != "" {
		s.rootPath = strings.TrimPrefix(initParams.RootURI, "file://")
	}
	
	// Check if Zed sends initializationOptions
	var rawInit struct {
		InitializationOptions json.RawMessage `json:"initializationOptions"`
	}
	if err := json.Unmarshal(params, &rawInit); err == nil && len(rawInit.InitializationOptions) > 0 {
		s.logger.Printf("InitializationOptions: %s", string(rawInit.InitializationOptions))
		
		// Parse the configuration directly from initializationOptions (no "mtlog" wrapper)
		var initConfig struct {
			SuppressedCodes        []string          `json:"suppressedCodes"`
			SeverityOverrides      map[string]string `json:"severityOverrides"`
			DisableAll            bool              `json:"disableAll"`
			CommonKeys            []string          `json:"commonKeys"`
			StrictMode            bool              `json:"strictMode"`
			IgnoreDynamicTemplates bool              `json:"ignoreDynamicTemplates"`
			// Analyzer-specific config
			AnalyzerConfig struct {
				Strict                 bool     `json:"strict"`
				CommonKeys             []string `json:"commonKeys"`
				DisabledChecks         []string `json:"disabledChecks"`
				IgnoreDynamicTemplates bool     `json:"ignoreDynamicTemplates"`
				StrictLoggerTypes      bool     `json:"strictLoggerTypes"`
				DowngradeErrors        bool     `json:"downgradeErrors"`
				CheckReservedProps     bool     `json:"checkReservedProps"`
				ReservedProps          []string `json:"reservedProps"`
			} `json:"analyzerConfig,omitempty"`
		}
		if err := json.Unmarshal(rawInit.InitializationOptions, &initConfig); err == nil {
			s.config.Mtlog.SuppressedCodes = initConfig.SuppressedCodes
			s.config.Mtlog.SeverityOverrides = initConfig.SeverityOverrides
			s.config.Mtlog.DisableAll = initConfig.DisableAll
			s.config.Mtlog.CommonKeys = initConfig.CommonKeys
			s.config.Mtlog.StrictMode = initConfig.StrictMode
			s.config.Mtlog.IgnoreDynamicTemplates = initConfig.IgnoreDynamicTemplates
			
			// Also copy analyzer config
			s.config.Analyzer = initConfig.AnalyzerConfig
			
			s.logger.Printf("Applied configuration from initializationOptions: suppressedCodes=%v, disableAll=%v", 
				s.config.Mtlog.SuppressedCodes, s.config.Mtlog.DisableAll)
		} else {
			s.logger.Printf("Failed to parse initializationOptions as config: %v", err)
		}
	} else {
		s.logger.Printf("No initializationOptions provided")
	}
	
	s.logger.Printf("Initialized with root: %s", s.rootPath)
	
	// Send initialize response
	result := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    1, // Full sync
				"save":      true,
			},
			"codeActionProvider": true,
		},
	}
	
	sendResponse(id, result)
}

// handleDidOpen processes the textDocument/didOpen notification when a file is opened.
// It immediately analyzes the file and publishes any diagnostics found.
func (s *Server) handleDidOpen(params json.RawMessage) {
	var openParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	
	if err := json.Unmarshal(params, &openParams); err != nil {
		s.logger.Printf("Error parsing didOpen params: %v", err)
		return
	}
	
	uri := openParams.TextDocument.URI
	path := uri
	if len(path) > 7 && path[:7] == "file://" {
		path = path[7:]
	}
	
	// Analyze the file using bundled analyzer
	diagnostics, fixes := s.runBundledAnalyzer(s.rootPath, path)
	
	// Cache the results
	s.diagnosticsCache[uri] = diagnostics
	s.fixesCache[uri] = fixes
	
	// Send diagnostics
	sendDiagnostics(uri, diagnostics)
}

// handleDidSave processes the textDocument/didSave notification when a file is saved.
// It re-analyzes the file and updates diagnostics.
func (s *Server) handleDidSave(params json.RawMessage) {
	var saveParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	
	if err := json.Unmarshal(params, &saveParams); err != nil {
		s.logger.Printf("Error parsing didSave params: %v", err)
		return
	}
	
	uri := saveParams.TextDocument.URI
	path := uri
	if len(path) > 7 && path[:7] == "file://" {
		path = path[7:]
	}
	
	// Re-analyze the file
	diagnostics, fixes := s.runBundledAnalyzer(s.rootPath, path)
	
	// Update caches
	s.diagnosticsCache[uri] = diagnostics
	s.fixesCache[uri] = fixes
	
	// Send updated diagnostics
	sendDiagnostics(uri, diagnostics)
}

// handleCodeAction processes the textDocument/codeAction request to provide code actions.
// It returns cached code actions (suggested fixes) for diagnostics at the requested position.
func (s *Server) handleCodeAction(id interface{}, params json.RawMessage) {
	var actionParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range struct {
			Start Position `json:"start"`
			End   Position `json:"end"`
		} `json:"range"`
		Context struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	
	if err := json.Unmarshal(params, &actionParams); err != nil {
		s.logger.Printf("Error parsing codeAction params: %v", err)
		sendResponse(id, []CodeAction{})
		return
	}
	
	uri := actionParams.TextDocument.URI
	actions := []CodeAction{}
	
	// Get cached fixes for this URI
	if fixesMap, ok := s.fixesCache[uri]; ok {
		s.logger.Printf("Found fixes cache for URI, keys: %v", getMapKeys(fixesMap))
		
		// For each diagnostic in the request, find matching fixes
		for _, diag := range actionParams.Context.Diagnostics {
			// Create the same key format used when storing fixes
			fullMessage := diag.Message
			if diag.Code != "" {
				fullMessage = fmt.Sprintf("[%s] %s", diag.Code, diag.Message)
			}
			diagKey := fmt.Sprintf("%d:%d-%s", diag.Range.Start.Line, diag.Range.Start.Character, fullMessage)
			
			s.logger.Printf("Looking for fixes with key: %s", diagKey)
			
			if fixes, found := fixesMap[diagKey]; found {
				s.logger.Printf("Found %d fixes for diagnostic", len(fixes))
				actions = append(actions, fixes...)
			}
		}
	} else {
		s.logger.Printf("No fixes cache found for URI: %s", uri)
	}
	
	s.logger.Printf("Returning %d code actions", len(actions))
	sendResponse(id, actions)
}

// shouldSuppressDiagnostic determines if a diagnostic should be suppressed based on configuration.
// It checks the global disable flag and the list of suppressed diagnostic codes.
func (s *Server) shouldSuppressDiagnostic(code string) bool {
	// Check global disable flag
	if s.config.Mtlog.DisableAll {
		return true
	}
	
	// Check if this specific code is suppressed
	for _, suppressedCode := range s.config.Mtlog.SuppressedCodes {
		if code == suppressedCode {
			return true
		}
	}
	
	return false
}

// byteOffsetToPosition converts a byte offset to LSP line/character position.
// LSP expects UTF-16 code unit positions, so this function properly handles
// multi-byte UTF-8 characters and surrogate pairs.
func byteOffsetToPosition(content []byte, offset int) (line, character int) {
	if offset > len(content) {
		offset = len(content)
	}
	
	line = 0
	character = 0
	bytePos := 0
	
	for bytePos < offset && bytePos < len(content) {
		r, size := utf8.DecodeRune(content[bytePos:])
		if r == '\n' {
			line++
			character = 0
		} else {
			// Count UTF-16 code units (most runes = 1, surrogates = 2)
			if r >= 0x10000 {
				character += 2 // Surrogate pair in UTF-16
			} else {
				character += 1
			}
		}
		bytePos += size
	}
	
	return line, character
}

// runBundledAnalyzer runs the analyzer directly on the specified file.
// It uses the bundled analyzer package instead of executing an external binary.
func (s *Server) runBundledAnalyzer(dir string, targetFile string) ([]Diagnostic, map[string][]CodeAction) {
	s.logger.Printf("Running bundled analyzer on %s", targetFile)
	
	diagnostics := []Diagnostic{}
	fixesMap := make(map[string][]CodeAction)
	
	// Read the file content for position conversion
	fileContent, err := os.ReadFile(targetFile)
	if err != nil {
		s.logger.Printf("Error reading file %s: %v", targetFile, err)
		return diagnostics, fixesMap
	}
	
	// Configure the analyzer based on our settings
	analyzerInstance := analyzer.Analyzer
	
	// Set analyzer flags based on configuration
	if s.config.Analyzer.Strict || s.config.Mtlog.StrictMode {
		analyzerInstance.Flags.Set("strict", "true")
	}
	
	// Check Analyzer config first, then fall back to Mtlog config
	// This allows flexible configuration through either the analyzer-specific or general settings
	if len(s.config.Analyzer.CommonKeys) > 0 {
		analyzerInstance.Flags.Set("common-keys", strings.Join(s.config.Analyzer.CommonKeys, ","))
	} else if len(s.config.Mtlog.CommonKeys) > 0 {
		analyzerInstance.Flags.Set("common-keys", strings.Join(s.config.Mtlog.CommonKeys, ","))
	}
	
	if len(s.config.Analyzer.DisabledChecks) > 0 {
		analyzerInstance.Flags.Set("disable", strings.Join(s.config.Analyzer.DisabledChecks, ","))
	}
	
	if s.config.Analyzer.IgnoreDynamicTemplates || s.config.Mtlog.IgnoreDynamicTemplates {
		analyzerInstance.Flags.Set("ignore-dynamic-templates", "true")
	}
	
	if s.config.Analyzer.StrictLoggerTypes {
		analyzerInstance.Flags.Set("strict-logger-types", "true")
	}
	
	if s.config.Analyzer.DowngradeErrors {
		analyzerInstance.Flags.Set("downgrade-errors", "true")
	}
	
	if s.config.Mtlog.DisableAll {
		analyzerInstance.Flags.Set("disable-all", "true")
	}
	
	// Add suppressed codes
	if len(s.config.Mtlog.SuppressedCodes) > 0 {
		analyzerInstance.Flags.Set("suppress", strings.Join(s.config.Mtlog.SuppressedCodes, ","))
	}
	
	// Load the package containing the target file
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedTypesSizes,
		Dir: dir,
		Env: os.Environ(),
		Tests: false,
	}
	
	// Load packages for the target file's directory
	pkgPath := filepath.Dir(targetFile)
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		s.logger.Printf("Error loading packages: %v", err)
		return diagnostics, fixesMap
	}
	
	// Process each package
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, err := range pkg.Errors {
				s.logger.Printf("Package error: %v", err)
			}
			continue
		}
		
		// Check if our target file is in this package
		fileInPackage := false
		for _, file := range pkg.CompiledGoFiles {
			if file == targetFile {
				fileInPackage = true
				break
			}
		}
		
		if !fileInPackage {
			continue
		}
		
		// Create an analysis pass for this package
		pass := &analysis.Pass{
			Analyzer:  analyzerInstance,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report: func(d analysis.Diagnostic) {
				// Only process diagnostics for our target file
				pos := pkg.Fset.Position(d.Pos)
				if pos.Filename != targetFile {
					return
				}
				
				// Extract diagnostic code from message if present
				message := d.Message
				code := ""
				if strings.HasPrefix(message, "[MTLOG") {
					if idx := strings.Index(message, "]"); idx > 0 {
						code = message[1:idx]
						message = strings.TrimSpace(message[idx+1:])
					}
				}
				
				// Check if this diagnostic should be suppressed
				if code != "" && s.shouldSuppressDiagnostic(code) {
					s.logger.Printf("Suppressing diagnostic %s", code)
					return
				}
				
				// Convert token positions to LSP positions
				startLine, startChar := byteOffsetToPosition(fileContent, pos.Offset)
				
				// For end position, use the diagnostic's end if available
				endPos := d.End
				if endPos == token.NoPos {
					// If no end position, use start + 1
					endPos = d.Pos + 1
				}
				endPosInfo := pkg.Fset.Position(endPos)
				endLine, endChar := byteOffsetToPosition(fileContent, endPosInfo.Offset)
				
				// Determine severity
				severity := 2 // Warning by default
				if strings.Contains(strings.ToLower(d.Message), "error") {
					severity = 1 // Error
				}
				
				// Apply severity overrides if configured
				if code != "" {
					if override, ok := s.config.Mtlog.SeverityOverrides[code]; ok {
						switch strings.ToLower(override) {
						case "error":
							severity = 1
						case "warning":
							severity = 2
						case "information", "info":
							severity = 3
						case "hint":
							severity = 4
						}
					}
				}
				
				diag := Diagnostic{
					Range: Range{
						Start: Position{Line: startLine, Character: startChar},
						End:   Position{Line: endLine, Character: endChar},
					},
					Severity: severity,
					Code:     code,
					Source:   "mtlog-analyzer",
					Message:  message,
				}
				
				diagnostics = append(diagnostics, diag)
				
				// Create diagnostic key for fixes lookup
				fullMessage := message
				if code != "" {
					fullMessage = fmt.Sprintf("[%s] %s", code, message)
				}
				diagKey := fmt.Sprintf("%d:%d-%s", startLine, startChar, fullMessage)
				
				// Process suggested fixes
				for _, fix := range d.SuggestedFixes {
					codeAction := CodeAction{
						Title:       fix.Message,
						Kind:        "quickfix",
						Diagnostics: []Diagnostic{diag},
						Edit: map[string]interface{}{
							"changes": map[string]interface{}{},
						},
					}
					
					// Convert text edits
					edits := []map[string]interface{}{}
					for _, edit := range fix.TextEdits {
						startPos := pkg.Fset.Position(edit.Pos)
						endPos := pkg.Fset.Position(edit.End)
						
						startLine, startChar := byteOffsetToPosition(fileContent, startPos.Offset)
						endLine, endChar := byteOffsetToPosition(fileContent, endPos.Offset)
						
						edits = append(edits, map[string]interface{}{
							"range": map[string]interface{}{
								"start": map[string]interface{}{
									"line":      startLine,
									"character": startChar,
								},
								"end": map[string]interface{}{
									"line":      endLine,
									"character": endChar,
								},
							},
							"newText": string(edit.NewText),
						})
					}
					
					if len(edits) > 0 {
						uri := "file://" + targetFile
						if changesMap, ok := codeAction.Edit["changes"].(map[string]interface{}); ok {
							changesMap[uri] = edits
						}
						fixesMap[diagKey] = append(fixesMap[diagKey], codeAction)
					}
				}
				
				// Add suppression code action if code is not empty
				if code != "" && !s.shouldSuppressDiagnostic(code) {
					settingsPath := filepath.Join(s.rootPath, ".zed", "settings.json")
					settingsURI := "file://" + settingsPath
					
					newText := fmt.Sprintf(`{
  "lsp": {
    "mtlog-analyzer": {
      "initialization_options": {
        "suppressedCodes": ["%s"]
      }
    }
  }
}`, code)
					
					suppressAction := CodeAction{
						Title: fmt.Sprintf("Suppress %s in workspace", code),
						Kind:  "quickfix.suppress.workspace",
						Diagnostics: []Diagnostic{diag},
						Edit: map[string]interface{}{
							"changes": map[string]interface{}{
								settingsURI: []map[string]interface{}{
									{
										"range": map[string]interface{}{
											"start": map[string]interface{}{"line": 0, "character": 0},
											"end":   map[string]interface{}{"line": 999999, "character": 0},
										},
										"newText": newText,
									},
								},
							},
						},
					}
					
					fixesMap[diagKey] = append(fixesMap[diagKey], suppressAction)
				}
			},
		}
		
		// Add inspect pass result (required by analyzer)
		_ = &analysis.Pass{
			Analyzer:  inspect.Analyzer,
			Fset:      pkg.Fset,
			Files:     pkg.Syntax,
			Pkg:       pkg.Types,
			TypesInfo: pkg.TypesInfo,
			Report:    func(d analysis.Diagnostic) {},
			ResultOf:  make(map[*analysis.Analyzer]interface{}),
		}
		
		// Run inspector
		inspector := inspector.New(pkg.Syntax)
		pass.ResultOf = map[*analysis.Analyzer]interface{}{
			inspect.Analyzer: inspector,
		}
		
		// Run the analyzer
		_, err := analyzerInstance.Run(pass)
		if err != nil {
			s.logger.Printf("Analyzer error: %v", err)
		}
	}
	
	s.logger.Printf("Found %d diagnostics for %s", len(diagnostics), targetFile)
	return diagnostics, fixesMap
}

// getMapKeys returns all keys from a map as a slice.
// Used for debugging to log available diagnostic keys in the fixes cache.
func getMapKeys(m map[string][]CodeAction) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}