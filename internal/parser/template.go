package parser

// MessageTemplate represents a parsed message template.
type MessageTemplate struct {
	// Raw is the original template string.
	Raw string
	
	// Tokens are the parsed tokens from the template.
	Tokens []MessageTemplateToken
}

// Render generates the final message using the provided properties.
func (mt *MessageTemplate) Render(properties map[string]interface{}) string {
	result := ""
	for _, token := range mt.Tokens {
		result += token.Render(properties)
	}
	return result
}