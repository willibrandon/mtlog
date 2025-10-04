package parser

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MessageTemplateToken represents a single token in a message template.
type MessageTemplateToken interface {
	// Render returns the string representation of the token using the provided properties.
	Render(properties map[string]any) string
}

// TextToken represents literal text in a message template.
type TextToken struct {
	// Text is the literal text content.
	Text string
}

// Render returns the literal text.
func (t *TextToken) Render(properties map[string]any) string {
	return t.Text
}

// PropertyToken represents a property placeholder in a message template.
type PropertyToken struct {
	// PropertyName is the name of the property.
	PropertyName string

	// Capturing specifies how the property should be captured.
	Capturing CapturingHint

	// Format specifies the format string, if any.
	Format string

	// Alignment specifies text alignment, if any.
	Alignment int
}

// Render returns the string representation of the property value.
func (p *PropertyToken) Render(properties map[string]any) string {
	if value, ok := properties[p.PropertyName]; ok {
		// Apply formatting
		formatted := p.formatValue(value)

		// Apply alignment
		if p.Alignment != 0 {
			formatted = p.applyAlignment(formatted)
		}

		return formatted
	}
	return "{" + p.PropertyName + "}"
}

// CapturingHint specifies how a property should be captured.
type CapturingHint int

const (
	// Default capturing uses Go's default string conversion (e.g., fmt.Sprintf("%v", value)).
	Default CapturingHint = iota

	// Stringify forces string conversion.
	Stringify

	// Capture captures object structure.
	Capture

	// AsScalar treats as scalar value.
	AsScalar
)

// formatValue formats a value according to the property's format string.
func (p *PropertyToken) formatValue(value any) string {
	// Handle JSON format for any type (including nil)
	if p.Format == "j" {
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("<json error: %v>", err)
		}
		return string(jsonBytes)
	}

	// Check for nil after JSON handling
	if value == nil {
		return ""
	}

	// Handle different value types with format strings
	switch v := value.(type) {
	case time.Time:
		// Handle time.Time explicitly to use custom formatting logic
		// instead of its default String() method (time.Time implements fmt.Stringer)
		if p.Format != "" {
			return p.formatTime(v)
		}
		return formatValue(value)
	case int, int8, int16, int32, int64:
		if p.Format != "" {
			return p.formatNumber(v)
		}
		return formatValue(value)
	case uint, uint8, uint16, uint32, uint64:
		if p.Format != "" {
			return p.formatNumber(v)
		}
		return formatValue(value)
	case float32, float64:
		if p.Format != "" {
			return p.formatFloat(v)
		}
		return formatValue(value)
	case fmt.Stringer:
		// Handle types that implement Stringer (includes Null and CapturedStruct from capture package)
		// This case is checked AFTER concrete types like time.Time to ensure proper formatting

		// Use reflection to identify the capture.Null type without importing it
		// This avoids import cycle issues while being more reliable than string comparison
		typeName := reflect.TypeOf(v).String()
		if typeName == "capture.Null" {
			// This is the special Null sentinel type - preserve its "nil" representation
			return v.String()
		}

		// For other Stringers (like CapturedStruct), use their string representation
		return v.String()
	case string:
		// Handle string formatting
		if p.Format == "l" {
			// Literal format - same as default (no quotes)
			return v
		}
		if p.Format == "q" {
			// Explicit quotes format
			return fmt.Sprintf("%q", v)
		}
		// Default behavior: no quotes (following modern Serilog)
		return v
	default:
		// For other types, use default formatting
		return formatValue(value)
	}
}

// formatNumber formats an integer according to the format string.
func (p *PropertyToken) formatNumber(value any) string {
	// Common formats:
	// "000" - pad with zeros to 3 digits
	// "X" or "x" - hexadecimal
	// "D" or "d" - decimal (default)

	var num int64
	switch v := value.(type) {
	case int:
		num = int64(v)
	case int8:
		num = int64(v)
	case int16:
		num = int64(v)
	case int32:
		num = int64(v)
	case int64:
		num = v
	case uint:
		num = int64(v)
	case uint8:
		num = int64(v)
	case uint16:
		num = int64(v)
	case uint32:
		num = int64(v)
	case uint64:
		num = int64(v)
	}

	// Handle padding with zeros
	if len(p.Format) > 0 && p.Format[0] == '0' {
		width := len(p.Format)
		return fmt.Sprintf("%0*d", width, num)
	}

	// Handle hex format
	if p.Format == "x" || p.Format == "X" {
		if p.Format == "x" {
			return fmt.Sprintf("%x", num)
		}
		return fmt.Sprintf("%X", num)
	}

	// Default decimal
	return strconv.FormatInt(num, 10)
}

// formatFloat formats a floating-point number according to the format string.
func (p *PropertyToken) formatFloat(value any) string {
	// Common formats:
	// "F2" or "f2" - fixed point with 2 decimal places
	// "E" or "e" - exponential notation
	// "G" or "g" - general format
	// "P" or "p" - percentage

	var num float64
	switch v := value.(type) {
	case float32:
		num = float64(v)
	case float64:
		num = v
	}

	format := strings.ToUpper(p.Format)

	// Extract precision if present
	precision := -1
	if len(format) > 1 {
		if p, err := strconv.Atoi(format[1:]); err == nil {
			precision = p
			format = format[:1]
		}
	}

	switch format {
	case "F":
		if precision >= 0 {
			return strconv.FormatFloat(num, 'f', precision, 64)
		}
		return strconv.FormatFloat(num, 'f', -1, 64)
	case "E":
		if precision >= 0 {
			return strconv.FormatFloat(num, 'e', precision, 64)
		}
		return strconv.FormatFloat(num, 'e', -1, 64)
	case "G":
		if precision >= 0 {
			return strconv.FormatFloat(num, 'g', precision, 64)
		}
		return strconv.FormatFloat(num, 'g', -1, 64)
	case "P":
		if precision >= 0 {
			return fmt.Sprintf("%.*f%%", precision, num*100)
		}
		return fmt.Sprintf("%.2f%%", num*100)
	default:
		return strconv.FormatFloat(num, 'g', -1, 64)
	}
}

// formatTime formats a time value according to the format string.
func (p *PropertyToken) formatTime(t time.Time) string {
	// Convert .NET-style format strings to Go format
	// Common patterns:
	// yyyy-MM-dd -> 2006-01-02
	// HH:mm:ss -> 15:04:05
	// yyyy-MM-dd HH:mm:ss -> 2006-01-02 15:04:05

	format := p.Format

	// Replace common .NET format patterns with Go equivalents
	replacements := []struct {
		from, to string
	}{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"MMMM", "January"},
		{"MMM", "Jan"},
		{"MM", "01"},
		{"M", "1"},
		{"dd", "02"},
		{"d", "2"},
		{"HH", "15"},
		{"H", "15"},
		{"hh", "03"},
		{"h", "3"},
		{"mm", "04"},
		{"m", "4"},
		{"ss", "05"},
		{"s", "5"},
		{"tt", "PM"},
		{"t", "PM"},
		{"fff", "000"},
		{"ff", "00"},
		{"f", "0"},
		{"zzz", "-07:00"},
		{"zz", "-07"},
		{"z", "-7"},
	}

	for _, r := range replacements {
		format = strings.ReplaceAll(format, r.from, r.to)
	}

	return t.Format(format)
}

// applyAlignment applies padding/alignment to a string.
func (p *PropertyToken) applyAlignment(s string) string {
	if p.Alignment == 0 {
		return s
	}

	width := p.Alignment
	if width < 0 {
		// Left align
		width = -width
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	} else {
		// Right align
		if len(s) >= width {
			return s
		}
		return strings.Repeat(" ", width-len(s)) + s
	}
}

func formatValue(value any) string {
	if value == nil {
		return "nil" // Go convention: nil without brackets
	}

	switch v := value.(type) {
	case []byte:
		// Special handling for byte slices - render as string if valid UTF-8
		if utf8.Valid(v) && len(v) > 0 {
			// Check if it's mostly printable
			printable := true
			for _, b := range v {
				if b < 32 && b != '\n' && b != '\r' && b != '\t' {
					printable = false
					break
				}
			}
			if printable {
				return string(v)
			}
		}
		// Fall back to byte array format
		return fmt.Sprintf("%v", v)
	case time.Time:
		// Standardize time format to RFC3339
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", value)
	}
}
