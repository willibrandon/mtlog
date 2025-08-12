package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// Parse parses a message template string into a MessageTemplate.
func Parse(template string) (*MessageTemplate, error) {
	if template == "" {
		return &MessageTemplate{
			Raw:    template,
			Tokens: []MessageTemplateToken{},
		}, nil
	}
	
	tokens := []MessageTemplateToken{}
	i := 0
	textStart := 0
	
	for i < len(template) {
		if template[i] == '{' {
			// Add any preceding text as a text token
			if i > textStart {
				tokens = append(tokens, &TextToken{Text: template[textStart:i]})
			}
			
			// Check for Go template syntax {{.Property}}
			if i+1 < len(template) && template[i+1] == '{' {
				// Look for }} to see if this might be a Go template
				closeIdx := strings.Index(template[i+2:], "}}")
				if closeIdx != -1 {
					// Found closing }}, check if it's a valid Go template
					content := template[i+2 : i+2+closeIdx]
					
					// Check for {{{{ which is escaped {{
					if len(content) >= 2 && content[0] == '{' && content[1] == '{' {
						// Escaped brace {{{{ -> {{
						tokens = append(tokens, &TextToken{Text: "{{"})
						i += 4
						textStart = i
						continue
					}
					
					// Check if content has no dots - this means it's escaped braces like {{text}}
					hasDot := false
					for _, r := range content {
						if r == '.' {
							hasDot = true
							break
						}
					}
					
					// If no dot found and not starting with @/$ and it's not a valid property name, treat as escaped braces
					if !hasDot && len(content) > 0 && content[0] != '@' && content[0] != '$' {
						// Check if it's a plain property name (valid identifier)
						isValidProp := isValidPropertyName(content)
						if !isValidProp {
							// This is escaped braces like {{double braces}}
							tokens = append(tokens, &TextToken{Text: "{"})
							tokens = append(tokens, &TextToken{Text: content})
							tokens = append(tokens, &TextToken{Text: "}"})
							i = i + 2 + closeIdx + 2 // Skip past }}
							textStart = i
							continue
						}
					}
					
					// Trim spaces to check for Go template patterns
					trimmedContent := strings.TrimSpace(content)
					
					// Check if it starts with . (Go template syntax)
					if len(trimmedContent) > 0 && trimmedContent[0] == '.' {
						// Check if original had spaces (invalid Go template)
						if content != trimmedContent {
							// Has spaces, treat as property token  
							propToken := parsePropertyToken(content)
							tokens = append(tokens, propToken)
							
							i = i + 2 + closeIdx + 2 // Skip past }}
							textStart = i
							continue
						}
						
						// This is Go template syntax {{.Property}}
						propertyContent := trimmedContent[1:] // Remove the leading .
						
						// For edge case {{.}} with no property name
						if propertyContent == "" {
							// Empty property name - use empty property token
							tokens = append(tokens, &PropertyToken{
								PropertyName: "",
								Capturing:    Default,
							})
						} else {
							// Parse property token (including capturing hints)
							propToken := parsePropertyToken(propertyContent)
							tokens = append(tokens, propToken)
						}
						
						i = i + 2 + closeIdx + 2 // Skip past }}
						textStart = i
						continue
					} else if len(content) > 0 && (content[0] == '@' || content[0] == '$') && len(content) > 1 && content[1] == '.' {
						// Handle {{@.Property}} or {{$.Property}}
						capturing := Default
						if content[0] == '@' {
							capturing = Capture
						} else if content[0] == '$' {
							capturing = AsScalar
						}
						
						propertyContent := content[2:] // Skip @. or $.
						propToken := parsePropertyToken(propertyContent)
						propToken.Capturing = capturing
						tokens = append(tokens, propToken)
						
						i = i + 2 + closeIdx + 2 // Skip past }}
						textStart = i
						continue
					} else {
						// Not a valid Go template (no dot), treat as regular property token
						propToken := parsePropertyToken(content)
						tokens = append(tokens, propToken)
						
						i = i + 2 + closeIdx + 2 // Skip past }}
						textStart = i
						continue
					}
				} else {
					// No closing }}, treat as text
					tokens = append(tokens, &TextToken{Text: template[i:]})
					textStart = len(template)
					break
				}
			}
			
			// Parse regular {Property} token
			propStart := i + 1
			propEnd := strings.IndexByte(template[propStart:], '}')
			if propEnd == -1 {
				// Unclosed property - treat as text
				tokens = append(tokens, &TextToken{Text: template[i:]})
				textStart = len(template) // Prevent adding duplicate token at end
				break
			}
			
			propEnd += propStart
			propertyContent := template[propStart:propEnd]
			
			// Parse property token
			propToken := parsePropertyToken(propertyContent)
			tokens = append(tokens, propToken)
			
			i = propEnd + 1
			textStart = i
		} else if template[i] == '}' {
			// Check for escaped brace
			if i+1 < len(template) && template[i+1] == '}' {
				// Add any preceding text
				if i > textStart {
					tokens = append(tokens, &TextToken{Text: template[textStart:i]})
				}
				tokens = append(tokens, &TextToken{Text: "}"})
				i += 2
				textStart = i
				continue
			}
			i++
		} else {
			i++
		}
	}
	
	// Add any remaining text
	if textStart < len(template) {
		tokens = append(tokens, &TextToken{Text: template[textStart:]})
	}
	
	return &MessageTemplate{
		Raw:    template,
		Tokens: tokens,
	}, nil
}

// parsePropertyToken parses the content of a property token.
func parsePropertyToken(content string) *PropertyToken {
	capturing := Default
	propertyName := content
	format := ""
	alignment := 0
	
	// Check for capturing prefix
	if len(content) > 0 {
		switch content[0] {
		case '@':
			capturing = Capture
			propertyName = content[1:]
		case '$':
			capturing = AsScalar
			propertyName = content[1:]
		}
	}
	
	// Parse format string and alignment
	// Format can be: {Name}, {Name:format}, {Name,alignment}, {Name,alignment:format}
	
	// Look for comma first (alignment)
	commaIdx := strings.IndexByte(propertyName, ',')
	colonIdx := strings.IndexByte(propertyName, ':')
	
	// Handle the case where we have both alignment and format
	if commaIdx != -1 && (colonIdx == -1 || commaIdx < colonIdx) {
		// We have alignment
		name := strings.TrimSpace(propertyName[:commaIdx])
		rest := propertyName[commaIdx+1:]
		
		// Check if there's also a format after the alignment
		colonInRest := strings.IndexByte(rest, ':')
		if colonInRest != -1 {
			// Parse alignment
			alignStr := strings.TrimSpace(rest[:colonInRest])
			if align, err := parseAlignment(alignStr); err == nil {
				alignment = align
			}
			// Parse format
			format = strings.TrimSpace(rest[colonInRest+1:])
		} else {
			// Just alignment
			if align, err := parseAlignment(strings.TrimSpace(rest)); err == nil {
				alignment = align
			}
		}
		propertyName = name
	} else if colonIdx != -1 {
		// Just format, no alignment
		name := strings.TrimSpace(propertyName[:colonIdx])
		format = strings.TrimSpace(propertyName[colonIdx+1:])
		propertyName = name
	} else {
		// No format or alignment
		propertyName = strings.TrimSpace(propertyName)
	}
	
	// Validate property name
	if !isValidPropertyName(propertyName) {
		// Invalid property name - return as-is
		return &PropertyToken{
			PropertyName: content,
			Capturing:    Default,
		}
	}
	
	return &PropertyToken{
		PropertyName: propertyName,
		Capturing:    capturing,
		Format:        format,
		Alignment:     alignment,
	}
}

// parseAlignment parses an alignment specification.
// Positive numbers mean right-align, negative mean left-align.
func parseAlignment(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	
	// Handle negative numbers (left alignment)
	negative := false
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}
	
	// Parse the number
	width := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid alignment: %s", s)
		}
		width = width*10 + int(ch-'0')
	}
	
	if negative {
		width = -width
	}
	return width, nil
}

// isValidPropertyName checks if a string is a valid property name.
func isValidPropertyName(name string) bool {
	if name == "" {
		return false
	}
	
	// Check if it's a numeric index (e.g., "0", "1", "2")
	if isNumericIndex(name) {
		return true
	}
	
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			// Allow letters, digits, underscores, hyphens, and dots (for OTEL compatibility)
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
				return false
			}
		}
	}
	
	return true
}

// isNumericIndex checks if a string is a numeric index like "0", "1", etc.
func isNumericIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ExtractPropertyNames returns all property names from a template.
func ExtractPropertyNames(template string) []string {
	mt, err := Parse(template)
	if err != nil {
		return []string{}
	}
	
	names := []string{}
	seen := make(map[string]bool)
	
	for _, token := range mt.Tokens {
		if prop, ok := token.(*PropertyToken); ok {
			if !seen[prop.PropertyName] {
				names = append(names, prop.PropertyName)
				seen[prop.PropertyName] = true
			}
		}
	}
	
	return names
}