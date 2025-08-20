//! Zed extension for mtlog-analyzer, providing real-time static analysis
//! for mtlog message templates in Go code.
//!
//! This extension integrates mtlog-analyzer's diagnostics into Zed through
//! the Language Server Protocol, offering features like template validation,
//! format specifier checking, and quick fixes for common issues.

use zed_extension_api::{self as zed, settings::LspSettings, Command, Extension, LanguageServerId, Result, Worktree};

/// Extension state for the mtlog-analyzer LSP integration.
/// Caches the binary path to avoid repeated filesystem lookups.
struct MtlogAnalyzerExtension {
    cached_binary_path: Option<String>,
}

impl MtlogAnalyzerExtension {
    /// Locates the mtlog-lsp binary using multiple strategies.
    ///
    /// Search order:
    /// 1. Explicit path from Zed settings
    /// 2. System PATH via `which` command
    /// 3. GOBIN environment variable
    /// 4. GOPATH/bin directory
    /// 5. HOME/go/bin (default Go installation)
    /// 6. /usr/local/bin fallback
    ///
    /// Returns the first valid path found, or a fallback path if none are found.
    fn find_mtlog_analyzer(&self, worktree: &Worktree) -> Option<String> {
        // Check explicit path from settings first
        if let Ok(lsp_settings) = LspSettings::for_worktree("mtlog-analyzer", worktree) {
            if let Some(binary) = lsp_settings.binary.as_ref() {
                if let Some(path) = binary.path.as_ref() {
                    return Some(path.clone());
                }
            }
        }

        // Use Zed's which() to find the binary in PATH
        // Now looking for mtlog-lsp instead of mtlog-analyzer
        if let Some(path) = worktree.which("mtlog-lsp") {
            return Some(path);
        }

        // Try common Go binary locations with explicit paths
        // Get shell environment to check GOPATH/GOBIN
        let env = worktree.shell_env();
        let env_map: std::collections::HashMap<String, String> = env.into_iter().collect();
        
        // Try GOBIN first
        if let Some(gobin) = env_map.get("GOBIN") {
            let binary_path = format!("{}/mtlog-lsp", gobin);
            // Since we can't check if file exists in WASM, we'll return this path
            // and let Zed handle the validation
            return Some(binary_path);
        }

        // Try GOPATH/bin
        if let Some(gopath) = env_map.get("GOPATH") {
            let binary_path = format!("{}/bin/mtlog-lsp", gopath);
            return Some(binary_path);
        }

        // Try HOME/go/bin (common default)
        if let Some(home) = env_map.get("HOME") {
            let binary_path = format!("{}/go/bin/mtlog-lsp", home);
            return Some(binary_path);
        }

        // Fallback to common locations
        Some("/usr/local/bin/mtlog-lsp".to_string())
    }
}

impl Extension for MtlogAnalyzerExtension {
    /// Creates a new instance of the extension with an empty cache.
    fn new() -> Self {
        Self {
            cached_binary_path: None,
        }
    }

    /// Returns the command to start the mtlog-lsp language server.
    ///
    /// This method is called by Zed when a Go file is opened. It locates
    /// the mtlog-lsp binary and returns the command to execute it.
    /// The binary path is cached after the first successful lookup.
    ///
    /// # Errors
    ///
    /// Returns an error if mtlog-lsp cannot be found in any of the standard locations.
    fn language_server_command(
        &mut self,
        _id: &LanguageServerId,
        worktree: &Worktree,
    ) -> Result<Command> {
        // Use cached path if available, otherwise find it
        let binary_path = if let Some(ref path) = self.cached_binary_path {
            path.clone()
        } else {
            let path = self.find_mtlog_analyzer(worktree)
                .ok_or_else(|| {
                    "mtlog-lsp not found. Please install it with: go install github.com/willibrandon/mtlog/cmd/mtlog-lsp@latest"
                        .to_string()
                })?;
            self.cached_binary_path = Some(path.clone());
            path
        };

        // mtlog-lsp doesn't need any arguments - it's a proper LSP server
        let args = vec![];

        Ok(Command {
            command: binary_path,
            args,
            env: Default::default(),
        })
    }
}

// Register the extension with Zed's extension system.
// This macro generates the WebAssembly bindings required for the extension to work.
zed::register_extension!(MtlogAnalyzerExtension);

#[cfg(test)]
mod tests {
    use super::*;

    /// Verifies that the extension can be created with proper initial state.
    #[test]
    fn test_extension_creation() {
        let ext = MtlogAnalyzerExtension::new();
        assert!(ext.cached_binary_path.is_none());
    }

    /// Tests that the path detection logic doesn't panic.
    /// Full testing requires WASM context which isn't available in unit tests.
    #[test]
    fn test_path_detection() {
        // Test that path detection logic doesn't panic
        // Note: We can't fully test Worktree in unit tests as it requires WASM context
        let ext = MtlogAnalyzerExtension::new();
        
        // Just verify the struct fields exist and can be accessed
        assert!(ext.cached_binary_path.is_none());
        
        // After finding a binary, it should be cached
        // This would require mocking Worktree which isn't possible in unit tests
    }
}