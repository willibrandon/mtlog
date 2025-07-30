package output

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/parser"
)

// Token represents a token in an output template
type Token interface {
	Render(event *core.LogEvent) string
}


// TextToken represents literal text in the template
type TextToken struct {
	Text string
}

func (t *TextToken) Render(event *core.LogEvent) string {
	return t.Text
}

// BuiltInToken represents a built-in template element like ${Message}
type BuiltInToken struct {
	Name   string
	Format string
}

func (t *BuiltInToken) Render(event *core.LogEvent) string {
	switch t.Name {
	case "Timestamp":
		return formatTimestamp(event.Timestamp, t.Format)
	case "Level":
		return formatLevel(event.Level, t.Format)
	case "Message":
		return formatMessage(event, t.Format)
	case "Exception":
		if event.Exception != nil {
			return event.Exception.Error()
		}
		return ""
	case "NewLine":
		return "\n"
	case "Properties":
		return formatProperties(event)
	default:
		// Unknown built-in
		return "${" + t.Name + "}"
	}
}

// PropertyToken represents a property reference with optional format
type PropertyToken struct {
	PropertyName string
	Format       string
}

func (t *PropertyToken) Render(event *core.LogEvent) string {
	// Check event properties
	if val, ok := event.Properties[t.PropertyName]; ok {
		return formatValue(val, t.Format)
	}
	// Property not found
	return "{" + t.PropertyName + "}"
}

// Template represents a parsed output template
type Template struct {
	Raw    string
	Tokens []Token
}

// Render renders the template with the given event
func (t *Template) Render(event *core.LogEvent) string {
	var sb strings.Builder
	for _, token := range t.Tokens {
		sb.WriteString(token.Render(event))
	}
	return sb.String()
}


// Parse parses an output template string
func Parse(template string) (*Template, error) {
	var tokens []Token
	runes := []rune(template)
	i := 0
	
	for i < len(runes) {
		// Look for built-in syntax ${...}
		if i+1 < len(runes) && runes[i] == '$' && runes[i+1] == '{' {
			// Find the closing brace
			start := i
			i += 2 // Skip ${  
			
			for i < len(runes) && runes[i] != '}' {
				i++
			}
			
			if i >= len(runes) {
				return nil, fmt.Errorf("unclosed built-in at position %d", start)
			}
			
			// Parse the built-in
			builtInStr := string(runes[start+2 : i])
			builtIn := parseBuiltIn(builtInStr)
			tokens = append(tokens, builtIn)
			i++ // Skip closing }
			continue
		}
		
		// Look for property start
		if runes[i] == '{' && i+1 < len(runes) && runes[i+1] == '{' {
			// Escaped brace
			tokens = append(tokens, &TextToken{Text: "{"})
			i += 2
			continue
		}
		
		if runes[i] == '{' {
			// Find the closing brace
			start := i
			depth := 1
			i++
			
			for i < len(runes) && depth > 0 {
				if runes[i] == '{' {
					depth++
				} else if runes[i] == '}' {
					depth--
				}
				i++
			}
			
			if depth != 0 {
				return nil, fmt.Errorf("unclosed property at position %d", start)
			}
			
			// Parse the property
			propStr := string(runes[start+1 : i-1])
			prop := parseProperty(propStr)
			tokens = append(tokens, prop)
		} else {
			// Collect text until next property or built-in
			start := i
			for i < len(runes) && runes[i] != '{' && !(i+1 < len(runes) && runes[i] == '$' && runes[i+1] == '{') {
				i++
			}
			tokens = append(tokens, &TextToken{Text: string(runes[start:i])})
		}
	}
	
	return &Template{
		Raw:    template,
		Tokens: tokens,
	}, nil
}

// parseBuiltIn parses a built-in reference like "Level:u3" or "Timestamp:HH:mm:ss"
func parseBuiltIn(str string) *BuiltInToken {
	parts := strings.SplitN(str, ":", 2)
	builtIn := &BuiltInToken{
		Name: strings.TrimSpace(parts[0]),
	}
	
	if len(parts) > 1 {
		builtIn.Format = strings.TrimSpace(parts[1])
	}
	
	return builtIn
}

// parseProperty parses a property reference like "UserId" or "Count:000"
func parseProperty(str string) *PropertyToken {
	parts := strings.SplitN(str, ":", 2)
	prop := &PropertyToken{
		PropertyName: strings.TrimSpace(parts[0]),
	}
	
	if len(parts) > 1 {
		prop.Format = strings.TrimSpace(parts[1])
	}
	
	return prop
}

// formatTimestamp formats a timestamp with the given format
func formatTimestamp(t time.Time, format string) string {
	if format == "" {
		format = "2006-01-02 15:04:05"
	}
	
	// Convert .NET format to Go format
	format = convertTimeFormat(format)
	return t.Format(format)
}

// convertTimeFormat converts .NET time format to Go time format
func convertTimeFormat(format string) string {
	// Replace longer patterns first to avoid partial replacements
	// e.g., "yyyy" should be replaced before "yy"
	replacements := []struct {
		old string
		new string
	}{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"MM", "01"},
		{"dd", "02"},
		{"HH", "15"},
		{"mm", "04"},
		{"ss", "05"},
		{"fff", "000"},
		{"ff", "00"},
		{"f", "0"},
		{"zzz", "-07:00"},
		{"zz", "-07"},
	}
	
	for _, r := range replacements {
		format = strings.ReplaceAll(format, r.old, r.new)
	}
	
	return format
}

// levelToString converts a log level to its string representation.
func levelToString(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "Verbose"
	case core.DebugLevel:
		return "Debug"
	case core.InformationLevel:
		return "Information"
	case core.WarningLevel:
		return "Warning"
	case core.ErrorLevel:
		return "Error"
	case core.FatalLevel:
		return "Fatal"
	default:
		return "Unknown"
	}
}

// formatLevel formats a log level with the given format
func formatLevel(level core.LogEventLevel, format string) string {
	switch format {
	case "u3": // uppercase 3-letter
		switch level {
		case core.VerboseLevel:
			return "VRB"
		case core.DebugLevel:
			return "DBG"
		case core.InformationLevel:
			return "INF"
		case core.WarningLevel:
			return "WRN"
		case core.ErrorLevel:
			return "ERR"
		case core.FatalLevel:
			return "FTL"
		default:
			return "UNK"
		}
	case "u": // uppercase full
		return strings.ToUpper(levelToString(level))
	case "l": // lowercase full
		return strings.ToLower(levelToString(level))
	default: // default format
		return levelToString(level)
	}
}

// formatMessage formats the rendered message with the given format
func formatMessage(event *core.LogEvent, format string) string {
	// Import parser at the top of the file
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		return event.MessageTemplate
	}
	
	message := tmpl.Render(event.Properties)
	
	switch format {
	case "lj": // left-justified (default)
		return message
	case "j": // JSON escaped
		return strings.ReplaceAll(strings.ReplaceAll(message, "\\", "\\\\"), "\"", "\\\"")
	default:
		return message
	}
}

// formatValue formats a value with the given format
func formatValue(val interface{}, format string) string {
	if format == "" {
		return fmt.Sprintf("%v", val)
	}
	
	// Handle numeric formats
	switch v := val.(type) {
	case int, int8, int16, int32, int64:
		return formatNumeric(v, format)
	case uint, uint8, uint16, uint32, uint64:
		return formatNumeric(v, format)
	case float32, float64:
		return formatFloat(v, format)
	case string:
		return formatString(v, format)
	case time.Time:
		return formatTimestamp(v, format)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// formatNumeric formats numeric values
func formatNumeric(val interface{}, format string) string {
	// Handle padding formats like "000" or "D3"
	if strings.HasPrefix(format, "D") {
		width := 0
		_, _ = fmt.Sscanf(format[1:], "%d", &width)
		return fmt.Sprintf("%0*d", width, val)
	}
	
	// Handle format as width specifier (e.g., "000" means width 3)
	if len(format) > 0 && format[0] == '0' {
		width := len(format)
		return fmt.Sprintf("%0*d", width, val)
	}
	
	return fmt.Sprintf("%v", val)
}

// formatFloat formats floating point values
func formatFloat(val interface{}, format string) string {
	var f float64
	switch v := val.(type) {
	case float32:
		f = float64(v)
	case float64:
		f = v
	default:
		return fmt.Sprintf("%v", val)
	}
	
	// Handle formats like "F2" (fixed-point with 2 decimals)
	if strings.HasPrefix(format, "F") {
		precision := 2
		if len(format) > 1 {
			_, _ = fmt.Sscanf(format[1:], "%d", &precision)
		}
		return fmt.Sprintf("%.*f", precision, f)
	}
	
	// Handle percentage format
	if format == "P" || strings.HasPrefix(format, "P") {
		precision := 0
		if len(format) > 1 {
			_, _ = fmt.Sscanf(format[1:], "%d", &precision)
		}
		return fmt.Sprintf("%.*f%%", precision, f*100)
	}
	
	return fmt.Sprintf("%v", f)
}

// formatString formats string values
func formatString(val string, format string) string {
	switch format {
	case "l": // lowercase
		return strings.ToLower(val)
	case "u": // uppercase
		return strings.ToUpper(val)
	case "j": // JSON escaped
		return strings.ReplaceAll(strings.ReplaceAll(val, "\\", "\\\\"), "\"", "\\\"")
	default:
		return val
	}
}

// formatProperties formats all properties as key=value pairs
func formatProperties(event *core.LogEvent) string {
	if len(event.Properties) == 0 {
		return ""
	}
	
	var pairs []string
	for k, v := range event.Properties {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	
	// Sort for consistent output
	sort.Strings(pairs)
	return strings.Join(pairs, " ")
}