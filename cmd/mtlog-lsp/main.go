// Package main implements mtlog-lsp, a Language Server Protocol wrapper for mtlog-analyzer.
//
// mtlog-lsp bridges the gap between mtlog-analyzer's go vet-style output and the
// Language Server Protocol, enabling rich IDE features like real-time diagnostics
// and code actions in LSP-compatible editors such as Zed.
//
// The server wraps mtlog-analyzer, converting its analysis output to LSP diagnostics
// and suggested fixes to LSP code actions, while handling the JSON-RPC communication
// protocol required by LSP.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// editContextLength is the number of characters to show before and after an edit location for debugging
	editContextLength = 20
)

// Server manages the LSP server state and caches.
// It maintains separate caches for diagnostics and their associated code actions
// to efficiently handle LSP requests without re-running the analyzer unnecessarily.
type Server struct {
	rootPath     string
	analyzerPath string
	logger       *log.Logger
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

// TextDocumentItem contains the full content and metadata of an opened text document.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpenTextDocumentParams contains the parameters for the textDocument/didOpen notification.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidSaveTextDocumentParams contains the parameters for the textDocument/didSave notification.
type DidSaveTextDocumentParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

// Diagnostic represents a problem found in the source code, such as a template/argument mismatch.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// Range defines a contiguous range between two positions in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a zero-based position in a text document using line and character offsets.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func main() {
	// Set up logging to stderr (stdout is for LSP communication)
	logger := log.New(os.Stderr, "[mtlog-lsp] ", log.LstdFlags)
	
	// Find mtlog-analyzer
	analyzerPath := findAnalyzer()
	if analyzerPath == "" {
		logger.Fatal("mtlog-analyzer not found in PATH or standard locations")
	}
	logger.Printf("Using analyzer: %s", analyzerPath)
	
	server := &Server{
		analyzerPath:     analyzerPath,
		logger:           logger,
		diagnosticsCache: make(map[string][]Diagnostic),
		fixesCache:       make(map[string]map[string][]CodeAction),
	}
	
	// Set up JSON-RPC communication
	reader := bufio.NewReader(os.Stdin)
	
	for {
		// Read Content-Length header
		contentLengthLine, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Printf("Error reading header: %v", err)
			continue
		}
		
		// Skip empty line after Content-Length
		_, _ = reader.ReadString('\n')
		
		// Parse Content-Length
		contentLength := 0
		if strings.HasPrefix(contentLengthLine, "Content-Length: ") {
			_, _ = fmt.Sscanf(contentLengthLine, "Content-Length: %d", &contentLength)
		}
		
		if contentLength == 0 {
			continue
		}
		
		// Read the JSON-RPC message
		msgBytes := make([]byte, contentLength)
		_, err = io.ReadFull(reader, msgBytes)
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
	s.diagnosticsCache = nil
	s.fixesCache = nil
}

// findAnalyzer locates the mtlog-analyzer binary in standard Go installation paths.
// It checks PATH, GOBIN, GOPATH/bin, ~/go/bin, and /usr/local/bin in order of preference.
// Returns the path to the analyzer or an empty string if not found.
func findAnalyzer() string {
	// Check common locations
	paths := []string{
		"mtlog-analyzer", // In PATH
	}
	
	// Add Go bin directories
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		paths = append(paths, filepath.Join(gobin, "mtlog-analyzer"))
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		paths = append(paths, filepath.Join(gopath, "bin", "mtlog-analyzer"))
	}
	if home := os.Getenv("HOME"); home != "" {
		paths = append(paths, filepath.Join(home, "go", "bin", "mtlog-analyzer"))
	}
	paths = append(paths, "/usr/local/bin/mtlog-analyzer")
	
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	return ""
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
// It triggers analysis for Go files and publishes the resulting diagnostics.
func (s *Server) handleDidOpen(params json.RawMessage) {
	var didOpen DidOpenTextDocumentParams
	if err := json.Unmarshal(params, &didOpen); err != nil {
		s.logger.Printf("Error parsing didOpen params: %v", err)
		return
	}
	
	// Only analyze Go files
	if didOpen.TextDocument.LanguageID != "go" {
		return
	}
	
	uri := didOpen.TextDocument.URI
	s.analyzAndPublish(uri)
}

// handleDidSave processes the textDocument/didSave notification when a file is saved.
// It re-runs the analyzer to detect any issues in the saved content.
func (s *Server) handleDidSave(params json.RawMessage) {
	var didSave DidSaveTextDocumentParams
	if err := json.Unmarshal(params, &didSave); err != nil {
		s.logger.Printf("Error parsing didSave params: %v", err)
		return
	}
	
	uri := didSave.TextDocument.URI
	s.analyzAndPublish(uri)
}

// analyzAndPublish runs mtlog-analyzer on the specified file and publishes diagnostics.
// It caches both diagnostics and their associated code actions for efficient retrieval.
func (s *Server) analyzAndPublish(uri string) {
	filePath := strings.TrimPrefix(uri, "file://")
	
	// Determine the directory to analyze
	dir := filepath.Dir(filePath)
	if s.rootPath != "" {
		// Analyze from the root if we have it
		dir = s.rootPath
	}
	
	s.logger.Printf("Analyzing %s in directory %s", filePath, dir)
	
	// Run mtlog-analyzer and get diagnostics with fixes
	diagnostics, fixes := s.runAnalyzer(dir, filePath)
	
	// Store diagnostics and fixes separately
	s.diagnosticsCache[uri] = diagnostics
	s.fixesCache[uri] = fixes
	
	// Publish diagnostics WITHOUT the Data field
	publishDiagnostics(uri, diagnostics)
}

// getMapKeys returns all keys from a map as a slice for debugging purposes.
func getMapKeys(m map[string][]CodeAction) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// handleCodeAction processes the textDocument/codeAction request to provide fixes for diagnostics.
// It retrieves cached code actions associated with the diagnostics in the requested range.
func (s *Server) handleCodeAction(id interface{}, params json.RawMessage) {
	// Parse code action params
	var codeActionParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range   Range `json:"range"`
		Context struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	
	if err := json.Unmarshal(params, &codeActionParams); err != nil {
		s.logger.Printf("Error parsing code action params: %v", err)
		sendResponse(id, []interface{}{})
		return
	}
	
	actions := []interface{}{}
	
	// Get cached fixes for this file
	if fixes, ok := s.fixesCache[codeActionParams.TextDocument.URI]; ok {
		s.logger.Printf("Found fixes cache for URI, keys: %v", getMapKeys(fixes))
		// For each diagnostic in the request context
		for _, diag := range codeActionParams.Context.Diagnostics {
			// Only handle our diagnostics
			if diag.Source != "mtlog-analyzer" {
				continue
			}
			
			// Reconstruct the key - need to add back the code if present
			fullMessage := diag.Message
			if diag.Code != "" {
				fullMessage = fmt.Sprintf("[%s] %s", diag.Code, diag.Message)
			}
			diagKey := fmt.Sprintf("%d:%d-%s", diag.Range.Start.Line, diag.Range.Start.Character, fullMessage)
			s.logger.Printf("Looking for fixes with key: %s", diagKey)
			
			// Get the cached fixes for this diagnostic
			if diagFixes, ok := fixes[diagKey]; ok {
				s.logger.Printf("Found %d fixes for diagnostic", len(diagFixes))
				for _, fix := range diagFixes {
					s.logger.Printf("Adding action: %+v", fix)
					actions = append(actions, fix)
				}
			} else {
				s.logger.Printf("No fixes found for key: %s", diagKey)
			}
		}
	} else {
		s.logger.Printf("No fixes cache found for URI: %s", codeActionParams.TextDocument.URI)
	}
	
	s.logger.Printf("Returning %d code actions", len(actions))
	sendResponse(id, actions)
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

// runAnalyzer executes mtlog-analyzer and converts its output to LSP diagnostics and code actions.
// It parses the analyzer's JSON output, filters diagnostics for the target file,
// and creates corresponding LSP structures with proper position conversion.
func (s *Server) runAnalyzer(dir string, targetFile string) ([]Diagnostic, map[string][]CodeAction) {
	// Read the file content for byte offset conversion
	fileContent, err := os.ReadFile(targetFile)
	if err != nil {
		s.logger.Printf("Error reading file %s: %v", targetFile, err)
		return nil, nil
	}
	
	// Run mtlog-analyzer on the directory
	cmd := exec.Command(s.analyzerPath, "-json", dir)
	cmd.Dir = dir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log the error but continue if we have output (analyzer returns non-zero for issues found)
		s.logger.Printf("Analyzer returned error (may be normal if issues found): %v", err)
		if len(output) == 0 {
			// No output at all means a real error
			s.logger.Printf("Analyzer failed with no output: %v", err)
			return nil, nil
		}
	}
	
	// Skip the "Args:" debug line if present
	outputStr := string(output)
	if idx := strings.Index(outputStr, "{"); idx > 0 {
		output = []byte(outputStr[idx:])
	}
	
	// Parse the JSON output - it's a map of package -> diagnostics
	var result map[string]map[string][]struct {
		Posn           string `json:"posn"`
		Message        string `json:"message"`
		SuggestedFixes []struct {
			Message string `json:"message"`
			Edits   []struct {
				Filename string `json:"filename"`
				Start    int    `json:"start"`
				End      int    `json:"end"`
				New      string `json:"new"`
			} `json:"edits"`
		} `json:"suggested_fixes,omitempty"`
	}
	
	if err := json.Unmarshal(output, &result); err != nil {
		s.logger.Printf("Error parsing analyzer JSON: %v, output: %s", err, string(output))
		return nil, nil
	}
	
	diagnostics := []Diagnostic{}
	fixesMap := make(map[string][]CodeAction)
	
	// Iterate through all packages and their diagnostics
	for _, pkg := range result {
		if mtlogDiags, ok := pkg["mtlog"]; ok {
			for _, issue := range mtlogDiags {
		
				// Parse position (format: "filename:line:col")
				parts := strings.SplitN(issue.Posn, ":", 3)
				if len(parts) < 3 {
					continue
				}
				
				// Check if this diagnostic is for our target file
				issueFile := filepath.Clean(parts[0])
				targetClean := filepath.Clean(targetFile)
				
				// Try exact match first, then base name match
				if issueFile != targetClean && filepath.Base(issueFile) != filepath.Base(targetClean) {
					// Also try if issueFile is relative and targetFile is absolute
					if !strings.HasSuffix(targetClean, issueFile) {
						continue
					}
				}
				
				lineNum, _ := strconv.Atoi(parts[1])
				colNum, _ := strconv.Atoi(parts[2])
				
				// LSP uses 0-based indexing
				lineNum = lineNum - 1
				colNum = colNum - 1
				
				if lineNum < 0 {
					lineNum = 0
				}
				if colNum < 0 {
					colNum = 0
				}
				
				// Extract diagnostic ID if present (e.g., "[MTLOG001] message")
				code := ""
				message := issue.Message
				if strings.HasPrefix(message, "[MTLOG") {
					if idx := strings.Index(message, "]"); idx > 0 {
						code = message[1:idx]
						message = strings.TrimSpace(message[idx+1:])
					}
				}
				
				// Create the diagnostic WITHOUT Data field
				diag := Diagnostic{
					Range: Range{
						Start: Position{Line: lineNum, Character: colNum},
						End:   Position{Line: lineNum, Character: colNum + 1}, // Single character for now
					},
					Severity: 2, // Warning
					Code:     code,
					Source:   "mtlog-analyzer",
					Message:  message,
				}
				diagnostics = append(diagnostics, diag)
				
				// Create a key for this diagnostic using full message
				diagKey := fmt.Sprintf("%d:%d-%s", lineNum, colNum, issue.Message)
				
				// Convert suggested fixes to code actions
				if len(issue.SuggestedFixes) > 0 {
					s.logger.Printf("Storing fixes for diagnostic key: %s", diagKey)
					for _, fix := range issue.SuggestedFixes {
						lspEdits := []map[string]interface{}{}
						for _, edit := range fix.Edits {
							// Convert byte offsets to line/character positions
							startLine, startChar := byteOffsetToPosition(fileContent, edit.Start)
							endLine, endChar := byteOffsetToPosition(fileContent, edit.End)
							
							// Debug: show the text around the edit position
							contextStart := edit.Start - editContextLength
							if contextStart < 0 {
								contextStart = 0
							}
							contextEnd := edit.End + editContextLength
							if contextEnd > len(fileContent) {
								contextEnd = len(fileContent)
							}
							context := string(fileContent[contextStart:contextEnd])
							
							s.logger.Printf("Edit: start=%d->(%d,%d), end=%d->(%d,%d), new=%q, context=%q", 
								edit.Start, startLine, startChar, edit.End, endLine, endChar, edit.New, context)
							
							lspEdits = append(lspEdits, map[string]interface{}{
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
								"newText": edit.New,
							})
						}
						
						// Log the actual edit we're creating
						editURI := "file://" + targetFile
						s.logger.Printf("Creating code action with URI: %s, edits: %v", editURI, lspEdits)
						
						codeAction := CodeAction{
							Title: fix.Message,
							Kind:  "quickfix",
							Diagnostics: []Diagnostic{diag},
							Edit: map[string]interface{}{
								"changes": map[string]interface{}{
									editURI: lspEdits,
								},
							},
						}
						
						fixesMap[diagKey] = append(fixesMap[diagKey], codeAction)
					}
				}
			}
		}
	}
	
	s.logger.Printf("Found %d diagnostics for %s", len(diagnostics), targetFile)
	return diagnostics, fixesMap
}

func publishDiagnostics(uri string, diagnostics []Diagnostic) {
	notification := JSONRPCMessage{
		Jsonrpc: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: json.RawMessage(mustMarshal(map[string]interface{}{
			"uri":         uri,
			"diagnostics": diagnostics,
		})),
	}
	
	sendMessage(notification)
}

func sendResponse(id interface{}, result interface{}) {
	response := JSONRPCMessage{
		Jsonrpc: "2.0",
		ID:      id,
	}
	
	if result != nil {
		response.Params = json.RawMessage(mustMarshal(map[string]interface{}{
			"result": result,
		}))
		// Actually, for responses we need a different structure
		responseMap := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  result,
		}
		sendMessage(responseMap)
		return
	}
	
	sendMessage(response)
}

// sendMessage sends a JSON-RPC message to the client via stdout.
// It prepends the required Content-Length header for LSP communication.
func sendMessage(msg interface{}) {
	msgBytes := mustMarshal(msg)
	content := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msgBytes), msgBytes)
	os.Stdout.Write([]byte(content))
}

// mustMarshal marshals a value to JSON, panicking on error.
// Used for internal structures that should always be marshalable.
func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}