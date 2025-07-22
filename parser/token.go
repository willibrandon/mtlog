package parser

import "fmt"

// MessageTemplateToken represents a single token in a message template.
type MessageTemplateToken interface {
	// Render returns the string representation of the token using the provided properties.
	Render(properties map[string]interface{}) string
}

// TextToken represents literal text in a message template.
type TextToken struct {
	// Text is the literal text content.
	Text string
}

// Render returns the literal text.
func (t *TextToken) Render(properties map[string]interface{}) string {
	return t.Text
}

// PropertyToken represents a property placeholder in a message template.
type PropertyToken struct {
	// PropertyName is the name of the property.
	PropertyName string
	
	// Destructuring specifies how the property should be destructured.
	Destructuring DestructuringHint
	
	// Format specifies the format string, if any.
	Format string
	
	// Alignment specifies text alignment, if any.
	Alignment int
}

// Render returns the string representation of the property value.
func (p *PropertyToken) Render(properties map[string]interface{}) string {
	if value, ok := properties[p.PropertyName]; ok {
		// TODO: Apply formatting and alignment
		return formatValue(value)
	}
	return "{" + p.PropertyName + "}"
}

// DestructuringHint specifies how a property should be destructured.
type DestructuringHint int

const (
	// Default destructuring uses ToString.
	Default DestructuringHint = iota
	
	// Stringify forces string conversion.
	Stringify
	
	// Destructure captures object structure.
	Destructure
	
	// AsScalar treats as scalar value.
	AsScalar
)

func formatValue(value interface{}) string {
	if value == nil {
		return ""
	}
	// TODO: Implement proper formatting
	return fmt.Sprintf("%v", value)
}