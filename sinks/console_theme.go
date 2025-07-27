package sinks

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	
	"github.com/willibrandon/mtlog/core"
)

// Color represents an ANSI color code.
type Color string

const (
	// Basic colors
	ColorReset   Color = "\033[0m"
	ColorBold    Color = "\033[1m"
	ColorDim     Color = "\033[2m"
	
	// Foreground colors
	ColorBlack   Color = "\033[30m"
	ColorRed     Color = "\033[31m"
	ColorGreen   Color = "\033[32m"
	ColorYellow  Color = "\033[33m"
	ColorBlue    Color = "\033[34m"
	ColorMagenta Color = "\033[35m"
	ColorCyan    Color = "\033[36m"
	ColorWhite   Color = "\033[37m"
	
	// Bright foreground colors
	ColorBrightBlack   Color = "\033[90m"
	ColorBrightRed     Color = "\033[91m"
	ColorBrightGreen   Color = "\033[92m"
	ColorBrightYellow  Color = "\033[93m"
	ColorBrightBlue    Color = "\033[94m"
	ColorBrightMagenta Color = "\033[95m"
	ColorBrightCyan    Color = "\033[96m"
	ColorBrightWhite   Color = "\033[97m"
	
	// Background colors
	ColorBgBlack   Color = "\033[40m"
	ColorBgRed     Color = "\033[41m"
	ColorBgGreen   Color = "\033[42m"
	ColorBgYellow  Color = "\033[43m"
	ColorBgBlue    Color = "\033[44m"
	ColorBgMagenta Color = "\033[45m"
	ColorBgCyan    Color = "\033[46m"
	ColorBgWhite   Color = "\033[47m"
)

// Ansi256Color creates an ANSI 256-color code.
func Ansi256Color(n int) Color {
	return Color(fmt.Sprintf("\033[38;5;%dm", n))
}

// Ansi256BgColor creates an ANSI 256-background color code.
func Ansi256BgColor(n int) Color {
	return Color(fmt.Sprintf("\033[48;5;%dm", n))
}

// ConsoleTheme defines the colors and formatting for console output.
type ConsoleTheme struct {
	// Level colors
	VerboseColor     Color
	DebugColor       Color
	InformationColor Color
	WarningColor     Color
	ErrorColor       Color
	FatalColor       Color
	
	// Element colors
	TimestampColor   Color
	MessageColor     Color
	PropertyKeyColor Color
	PropertyValColor Color
	BracketColor     Color // Color for brackets and delimiters
	
	// Formatting
	LevelFormat      string // Format string for level, e.g., "[%s]" or "%s:"
	TimestampFormat  string // Time format string
	PropertyFormat   string // Format for properties, e.g., "%s=%v"
}

// DefaultTheme returns the default console theme.
func DefaultTheme() *ConsoleTheme {
	return &ConsoleTheme{
		VerboseColor:     ColorBrightBlack,
		DebugColor:       ColorCyan,
		InformationColor: ColorGreen,
		WarningColor:     ColorYellow,
		ErrorColor:       ColorRed,
		FatalColor:       ColorBrightRed + ColorBold,
		
		TimestampColor:   ColorBrightBlack,
		MessageColor:     ColorReset,
		PropertyKeyColor: ColorBrightBlue,
		PropertyValColor: ColorReset,
		BracketColor:     ColorBrightBlack,
		
		LevelFormat:     "[%s]",
		TimestampFormat: "2006-01-02 15:04:05.000",
		PropertyFormat:  "%s=%v",
	}
}

// LiteTheme returns a minimalist theme with subtle colors.
func LiteTheme() *ConsoleTheme {
	return &ConsoleTheme{
		VerboseColor:     ColorDim,
		DebugColor:       ColorDim,
		InformationColor: ColorReset,
		WarningColor:     ColorYellow,
		ErrorColor:       ColorRed,
		FatalColor:       ColorRed + ColorBold,
		
		TimestampColor:   ColorDim,
		MessageColor:     ColorReset,
		PropertyKeyColor: ColorDim,
		PropertyValColor: ColorReset,
		BracketColor:     ColorDim,
		
		LevelFormat:     "%s",
		TimestampFormat: "15:04:05",
		PropertyFormat:  "%s=%v",
	}
}

// DevTheme returns a developer-friendly theme with more information.
func DevTheme() *ConsoleTheme {
	return &ConsoleTheme{
		VerboseColor:     ColorMagenta,
		DebugColor:       ColorCyan,
		InformationColor: ColorBrightGreen,
		WarningColor:     ColorBrightYellow,
		ErrorColor:       ColorBrightRed,
		FatalColor:       ColorBgRed + ColorWhite + ColorBold,
		
		TimestampColor:   ColorBrightCyan,
		MessageColor:     ColorReset,
		PropertyKeyColor: ColorBrightMagenta,
		PropertyValColor: ColorBrightBlue,
		BracketColor:     ColorBrightBlack,
		
		LevelFormat:     "[%-5s]", // Fixed width for alignment
		TimestampFormat: "2006-01-02 15:04:05.000",
		PropertyFormat:  "%s: %v",
	}
}

// NoColorTheme returns a theme without any colors.
func NoColorTheme() *ConsoleTheme {
	return &ConsoleTheme{
		VerboseColor:     "",
		DebugColor:       "",
		InformationColor: "",
		WarningColor:     "",
		ErrorColor:       "",
		FatalColor:       "",
		
		TimestampColor:   "",
		MessageColor:     "",
		PropertyKeyColor: "",
		PropertyValColor: "",
		BracketColor:     "",
		
		LevelFormat:     "[%s]",
		TimestampFormat: "2006-01-02 15:04:05.000",
		PropertyFormat:  "%s=%v",
	}
}

// LiterateTheme returns the Serilog Literate theme using ANSI 256 colors.
// This theme provides readable, non-garish colors that are easy on the eyes.
func LiterateTheme() *ConsoleTheme {
	return &ConsoleTheme{
		// Level colors from Serilog Literate (based on actual screenshot)
		VerboseColor:     Ansi256Color(7),   // Gray
		DebugColor:       Ansi256Color(7),   // Gray  
		InformationColor: Ansi256Color(15),  // Bright white
		WarningColor:     Ansi256Color(11),  // Yellow
		ErrorColor:       Ansi256Color(9),   // Red text (NOT background!)
		FatalColor:       Ansi256Color(9),   // Red text
		
		// Element colors
		TimestampColor:   Ansi256Color(7),   // Gray (SecondaryText)
		MessageColor:     Ansi256Color(15),  // Bright white (Text)
		PropertyKeyColor: Ansi256Color(7),   // Gray (Name)
		PropertyValColor: Ansi256Color(51),  // Cyan (for properties in messages)
		BracketColor:     Ansi256Color(238), // Darker gray for brackets
		
		LevelFormat:     "%s",
		TimestampFormat: "15:04:05",
		PropertyFormat:  "%s=%v",
	}
}

// Literate8ColorTheme returns a Literate-inspired theme using only basic 8 ANSI colors.
// This is useful for terminals that don't support 256 colors.
func Literate8ColorTheme() *ConsoleTheme {
	return &ConsoleTheme{
		// Level colors using basic ANSI colors
		VerboseColor:     ColorBrightBlack,  // Gray substitute
		DebugColor:       ColorBrightBlack,  // Gray substitute
		InformationColor: ColorWhite,        // White
		WarningColor:     ColorYellow,       // Yellow
		ErrorColor:       ColorRed,          // Red
		FatalColor:       ColorRed,          // Red
		
		// Element colors
		TimestampColor:   ColorBrightBlack,  // Gray substitute
		MessageColor:     ColorWhite,        // White
		PropertyKeyColor: ColorBrightBlack,  // Gray substitute
		PropertyValColor: ColorCyan,         // Cyan
		BracketColor:     ColorBrightBlack,  // Gray substitute
		
		LevelFormat:     "%s",
		TimestampFormat: "15:04:05",
		PropertyFormat:  "%s=%v",
	}
}

// GetLevelColor returns the color for a specific log level.
func (t *ConsoleTheme) GetLevelColor(level core.LogEventLevel) Color {
	switch level {
	case core.VerboseLevel:
		return t.VerboseColor
	case core.DebugLevel:
		return t.DebugColor
	case core.InformationLevel:
		return t.InformationColor
	case core.WarningLevel:
		return t.WarningColor
	case core.ErrorLevel:
		return t.ErrorColor
	case core.FatalLevel:
		return t.FatalColor
	default:
		return ColorReset
	}
}

// shouldUseColor determines if color output should be used.
func shouldUseColor(w io.Writer) bool {
	// Check MTLOG_FORCE_COLOR first
	if forceColor := os.Getenv("MTLOG_FORCE_COLOR"); forceColor != "" {
		switch strings.ToLower(forceColor) {
		case "none", "0", "false", "off":
			return false
		case "8", "16", "256", "true", "on":
			return true
		}
	}
	
	// Check if NO_COLOR env var is set
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	
	// On Windows, check if we're in a terminal that supports ANSI
	if runtime.GOOS == "windows" {
		// Check for Windows Terminal, ConEmu, or other modern terminals
		if _, ok := os.LookupEnv("WT_SESSION"); ok {
			return true
		}
		if _, ok := os.LookupEnv("ConEmuPID"); ok {
			return true
		}
		// Default to no color on Windows unless explicitly enabled
		return false
	}
	
	// On Unix-like systems, check if output is a terminal
	// This is a simplified check - in production you might use isatty
	return true
}

// supports256Colors checks if the terminal supports 256 colors.
func supports256Colors() bool {
	// Check MTLOG_FORCE_COLOR first
	if forceColor := os.Getenv("MTLOG_FORCE_COLOR"); forceColor != "" {
		switch strings.ToLower(forceColor) {
		case "8", "16":
			return false // Force basic colors
		case "256":
			return true // Force 256 colors
		}
	}
	
	// Check TERM environment variable
	term := os.Getenv("TERM")
	if term == "" {
		return false
	}
	
	// Common terminals that support 256 colors
	if strings.Contains(term, "256color") ||
		strings.Contains(term, "256colour") {
		return true
	}
	
	// Check COLORTERM for truecolor/256color support
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return true
	}
	
	// Windows Terminal and ConEmu support 256 colors
	if runtime.GOOS == "windows" {
		if _, ok := os.LookupEnv("WT_SESSION"); ok {
			return true
		}
		if _, ok := os.LookupEnv("ConEmuPID"); ok {
			return true
		}
	}
	
	// Default to basic colors
	return false
}

// AutoLiterateTheme returns LiterateTheme for 256-color terminals or Literate8ColorTheme otherwise.
func AutoLiterateTheme() *ConsoleTheme {
	if supports256Colors() {
		return LiterateTheme()
	}
	return Literate8ColorTheme()
}

// colorize applies color to a string if colors are enabled.
func colorize(s string, color Color, useColor bool) string {
	if !useColor || color == "" {
		return s
	}
	return string(color) + s + string(ColorReset)
}