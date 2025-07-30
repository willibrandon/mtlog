package parser

import (
	"fmt"
	"strings"
)

// ValidationError represents a template validation error
type ValidationError struct {
	Message  string
	Position int
	Property string
}

func (e ValidationError) Error() string {
	if e.Position > 0 {
		return fmt.Sprintf("%s at position %d", e.Message, e.Position)
	}
	return e.Message
}

// ValidateTemplate validates a message template and returns any errors found.
// This is used for runtime validation and selflog instrumentation.
func ValidateTemplate(template string) error {
	if template == "" {
		return nil
	}

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
				// Validate property
				prop := template[propertyStart:i]
				if err := validateProperty(prop); err != nil {
					return ValidationError{
						Message:  err.Error(),
						Position: propertyStart,
						Property: prop,
					}
				}
				inProperty = false
			}
		}
	}

	// Check for unclosed property
	if inProperty {
		prop := template[propertyStart:]
		return ValidationError{
			Message:  "unclosed property",
			Position: propertyStart,
			Property: prop,
		}
	}

	return nil
}

// validateProperty validates a single property content
func validateProperty(prop string) error {
	if prop == "" {
		return fmt.Errorf("empty property name")
	}

	// Remove format specifier and alignment for validation
	propName := prop
	if idx := strings.IndexAny(prop, ",:"); idx != -1 {
		propName = prop[:idx]
	}

	// Remove capturing hints
	originalProp := propName
	propName = strings.TrimPrefix(propName, "@")
	propName = strings.TrimPrefix(propName, "$")
	propName = strings.TrimSpace(propName)

	if propName == "" {
		hint := ""
		if strings.HasPrefix(originalProp, "@") {
			hint = "@"
		} else if strings.HasPrefix(originalProp, "$") {
			hint = "$"
		}
		return fmt.Errorf("empty property name after capturing hint '%s'", hint)
	}

	// Check for spaces in property name
	if strings.Contains(propName, " ") {
		return fmt.Errorf("property name contains spaces")
	}

	// Check if property is just dots with no actual name
	allDots := true
	for _, r := range propName {
		if r != '.' {
			allDots = false
			break
		}
	}
	if allDots {
		return fmt.Errorf("property name cannot be only dots")
	}

	// Check if starts with number
	if propName[0] >= '0' && propName[0] <= '9' {
		return fmt.Errorf("property name starts with a number")
	}

	return nil
}