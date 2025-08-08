package analyzer

import (
	"fmt"
	"strings"
)

// extractProperties parses a message template and extracts all property names.
// It handles escaped braces ({{) and returns property names including any format specifiers.
func extractProperties(template string) ([]string, error) {
	var properties []string
	inProperty := false
	propertyStart := 0
	braceDepth := 0
	
	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			// Check for escaped brace
			if i+1 < len(template) && template[i+1] == '{' {
				i++ // Skip escaped brace
				continue
			}
			if !inProperty {
				inProperty = true
				propertyStart = i + 1
				braceDepth = 1
			} else {
				braceDepth++
			}
		} else if template[i] == '}' && inProperty {
			braceDepth--
			if braceDepth == 0 {
				// Extract property name
				prop := template[propertyStart:i]
				if prop != "" {
					properties = append(properties, prop)
				}
				inProperty = false
			}
		}
	}
	
	// Check for unclosed property
	if inProperty {
		return nil, fmt.Errorf("unclosed property brace at position %d", propertyStart-1)
	}
	
	return properties, nil
}

// validFormats defines the known format specifiers for mtlog
var validFormats = map[string]bool{
	// Number formats
	"000": true, "0000": true, "00000": true,
	// Float formats
	"F": true, "F0": true, "F1": true, "F2": true, "F3": true, "F4": true,
	// Percentage
	"P": true, "P0": true, "P1": true, "P2": true,
	// Exponential
	"E": true, "E0": true, "E1": true, "E2": true,
	// General
	"G": true, "G0": true, "G1": true, "G2": true,
	// Hex
	"X": true, "X2": true, "X4": true, "X8": true,
	"x": true, "x2": true, "x4": true, "x8": true,
}

// validateFormatSpecifier checks if a property's format specifier is valid.
// Returns nil if the format is valid or no format specifier is present.
func validateFormatSpecifier(property string, config *Config) error {
	// Split property and format specifier
	parts := strings.SplitN(property, ":", 2)
	if len(parts) < 2 {
		return nil // No format specifier
	}
	
	format := parts[1]
	
	// Check if it's a known format
	if validFormats[format] {
		return nil
	}
	
	// Check if it's a time format (contains time format characters)
	timeChars := []string{"HH", "mm", "ss", "yyyy", "MM", "dd"}
	for _, tc := range timeChars {
		if strings.Contains(format, tc) {
			return nil
		}
	}
	
	// Check if it's an alignment specifier
	// Alignment should be like "-10" (left align) or "10" (right align with spaces)
	// But single digits like "3" are more likely intended as padding "000"
	if len(format) > 0 {
		if format[0] == '-' && len(format) > 1 {
			// Check if rest is digits (e.g., "-10")
			isAlign := true
			for i := 1; i < len(format); i++ {
				if format[i] < '0' || format[i] > '9' {
					isAlign = false
					break
				}
			}
			if isAlign {
				return nil
			}
		} else if len(format) >= 2 && format[0] >= '1' && format[0] <= '9' {
			// Multi-digit alignment like "10", "20" etc
			isAlign := true
			for i := 1; i < len(format); i++ {
				if format[i] < '0' || format[i] > '9' {
					isAlign = false
					break
				}
			}
			if isAlign {
				return nil
			}
		}
		// Single digit is not considered alignment in strict mode
		// It's probably meant to be padding like "000"
	}
	
	// In strict mode, unknown formats are errors
	if config.StrictMode {
		return fmt.Errorf("unknown format specifier: %s", format)
	}
	
	// In lenient mode, unknown formats are allowed
	// This behavior is documented in the README
	return nil
}