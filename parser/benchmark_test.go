package parser

import (
	"testing"
)

func BenchmarkParseSimple(b *testing.B) {
	template := "This is a simple message with no properties"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = Parse(template)
	}
}

func BenchmarkParseSingleProperty(b *testing.B) {
	template := "User {UserId} logged in"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = Parse(template)
	}
}

func BenchmarkParseMultipleProperties(b *testing.B) {
	template := "User {UserId} from {Country} using {Browser} on {OS} performed {Action}"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = Parse(template)
	}
}

func BenchmarkParseWithDestructuring(b *testing.B) {
	template := "Processing {@User} with {$Exception} at {Timestamp}"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = Parse(template)
	}
}

func BenchmarkParseEscapedBraces(b *testing.B) {
	template := "Use {{double braces}} to escape {Property} in templates"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = Parse(template)
	}
}

func BenchmarkExtractPropertyNames(b *testing.B) {
	template := "User {UserId} from {Country} performed {Action} on {Date}"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = ExtractPropertyNames(template)
	}
}

func BenchmarkRenderSimple(b *testing.B) {
	template := "User {UserId} logged in"
	mt, _ := Parse(template)
	properties := map[string]interface{}{
		"UserId": 123,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = mt.Render(properties)
	}
}

func BenchmarkRenderComplex(b *testing.B) {
	template := "User {UserId} from {Country} using {Browser} on {OS} performed {Action} at {Timestamp}"
	mt, _ := Parse(template)
	properties := map[string]interface{}{
		"UserId":    123,
		"Country":   "USA",
		"Browser":   "Chrome",
		"OS":        "Windows",
		"Action":    "login",
		"Timestamp": "2024-01-15T10:30:45Z",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = mt.Render(properties)
	}
}

func BenchmarkIsValidPropertyName(b *testing.B) {
	names := []string{
		"UserId",
		"user_id",
		"userName123",
		"_privateField",
		"invalid-name",
		"123invalid",
		"",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = isValidPropertyName(names[i%len(names)])
	}
}