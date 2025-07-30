package parser

import (
	"strings"
	"testing"
)

func TestParseWithDottedPropertyNames(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		wantProps    []string
		wantTokens   int
		wantErr      bool
		description  string
	}{
		{
			name:        "simple dotted property",
			template:    "HTTP request {http.method} to {http.url}",
			wantProps:   []string{"http.method", "http.url"},
			wantTokens:  4, // "HTTP request ", {http.method}, " to ", {http.url}
			description: "Should parse OTEL-style dotted properties",
		},
		{
			name:        "multiple dots in property",
			template:    "Database {db.system.version} query took {db.query.duration.ms}ms",
			wantProps:   []string{"db.system.version", "db.query.duration.ms"},
			wantTokens:  5,
			description: "Should handle multiple dots in property names",
		},
		{
			name:        "dots with capturing",
			template:    "User {@user.profile} made request {$http.status.code}",
			wantProps:   []string{"user.profile", "http.status.code"},
			wantTokens:  4, // "User ", {@user.profile}, " made request ", {$http.status.code}
			description: "Should handle dots with capturing hints",
		},
		{
			name:        "dots with format specifiers",
			template:    "Duration: {http.duration.ms:F2}ms, Code: {http.status.code:000}",
			wantProps:   []string{"http.duration.ms", "http.status.code"},
			wantTokens:  4, // "Duration: ", {http.duration.ms:F2}, "ms, Code: ", {http.status.code:000}
			description: "Should handle dots with format specifiers",
		},
		{
			name:        "dots with alignment",
			template:    "Method: {http.method,10} Status: {http.status.code,-5}",
			wantProps:   []string{"http.method", "http.status.code"},
			wantTokens:  4, // "Method: ", {http.method,10}, " Status: ", {http.status.code,-5}
			description: "Should handle dots with alignment",
		},
		{
			name:        "dots with both alignment and format",
			template:    "Duration: {http.duration.ms,8:F2}ms",
			wantProps:   []string{"http.duration.ms"},
			wantTokens:  3,
			description: "Should handle dots with both alignment and format",
		},
		{
			name:        "Go template syntax with dots",
			template:    "User {{.user.name}} logged in from {{.client.ip}}",
			wantProps:   []string{"user.name", "client.ip"},
			wantTokens:  4, // "User ", {{.user.name}}, " logged in from ", {{.client.ip}}
			description: "Should handle Go template syntax with dotted properties",
		},
		{
			name:        "Go template with capturing and dots",
			template:    "Profile: {{@.user.profile.data}}",
			wantProps:   []string{"user.profile.data"},
			wantTokens:  2,
			description: "Should handle Go template with capturing and dots",
		},
		{
			name:        "mixed regular and dotted properties",
			template:    "User {UserId} made {http.method} request to {http.url}",
			wantProps:   []string{"UserId", "http.method", "http.url"},
			wantTokens:  6, // "User ", {UserId}, " made ", {http.method}, " request to ", {http.url}
			description: "Should handle mix of regular and dotted properties",
		},
		{
			name:        "property starting with dot is invalid",
			template:    "Invalid {.http.method}",
			wantProps:   []string{".http.method"},
			wantTokens:  2, // "Invalid ", {.http.method}
			description: "Property starting with dot should be treated as-is",
		},
		{
			name:        "property ending with dot",
			template:    "Property {http.method.} value",
			wantProps:   []string{"http.method."},
			wantTokens:  3,
			description: "Property ending with dot is valid",
		},
		{
			name:        "consecutive dots",
			template:    "Property {http..method} value",
			wantProps:   []string{"http..method"},
			wantTokens:  3,
			description: "Consecutive dots are allowed",
		},
		{
			name:        "OTEL standard attributes",
			template:    "Service {service.name} version {service.version} in {service.namespace}",
			wantProps:   []string{"service.name", "service.version", "service.namespace"},
			wantTokens:  6, // "Service ", {service.name}, " version ", {service.version}, " in ", {service.namespace}
			description: "Should handle standard OTEL service attributes",
		},
		{
			name:        "OTEL HTTP attributes",
			template:    "{http.method} {http.route} returned {http.response.status_code}",
			wantProps:   []string{"http.method", "http.route", "http.response.status_code"},
			wantTokens:  5,
			description: "Should handle OTEL HTTP semantic conventions",
		},
		{
			name:        "OTEL database attributes",
			template:    "Query to {db.system} database {db.name} on {db.connection_string}",
			wantProps:   []string{"db.system", "db.name", "db.connection_string"},
			wantTokens:  6, // "Query to ", {db.system}, " database ", {db.name}, " on ", {db.connection_string}
			description: "Should handle OTEL database attributes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := Parse(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Check token count
			if len(mt.Tokens) != tt.wantTokens {
				t.Errorf("Parse() got %d tokens, want %d", len(mt.Tokens), tt.wantTokens)
			}

			// Extract and check property names
			gotProps := ExtractPropertyNames(tt.template)
			if len(gotProps) != len(tt.wantProps) {
				t.Errorf("Parse() got properties %v, want %v", gotProps, tt.wantProps)
				return
			}

			// Check each property
			for i, want := range tt.wantProps {
				if i >= len(gotProps) || gotProps[i] != want {
					t.Errorf("Parse() property[%d] = %v, want %v", i, gotProps[i], want)
				}
			}

			// Verify property tokens have correct names
			propIndex := 0
			for _, token := range mt.Tokens {
				if prop, ok := token.(*PropertyToken); ok {
					if propIndex >= len(tt.wantProps) {
						t.Errorf("Parse() found extra property token: %v", prop.PropertyName)
						continue
					}
					if prop.PropertyName != tt.wantProps[propIndex] {
						t.Errorf("Parse() property token = %v, want %v", prop.PropertyName, tt.wantProps[propIndex])
					}
					propIndex++
				}
			}
		})
	}
}

func TestIsValidPropertyNameWithDots(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
	}{
		// Valid cases with dots
		{"simple dot", "http.method", true},
		{"multiple dots", "http.request.duration.ms", true},
		{"underscore and dot", "http_request.method", true},
		{"hyphen and dot", "http-request.method", true},
		{"mixed separators", "http_request.status-code", true},
		
		// Edge cases that are valid
		{"ending with dot", "http.method.", true},
		{"consecutive dots", "http..method", true},
		{"dot after underscore", "http_.method", true},
		{"dot after hyphen", "http-.method", true},
		
		// Invalid cases
		{"empty string", "", false},
		{"starts with dot", ".method", false},
		{"starts with number", "123.method", false},
		{"starts with hyphen", "-http.method", false},
		{"contains space", "http .method", false},
		{"contains special char", "http@method", false},
		{"just dot", ".", false},
		{"just dots", "..", false},
		
		// Standard valid cases (no dots)
		{"simple", "UserId", true},
		{"with underscore", "user_id", true},
		{"with hyphen", "user-id", true},
		{"with numbers", "user123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPropertyName(tt.input); got != tt.want {
				t.Errorf("isValidPropertyName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateTemplateWithDots(t *testing.T) {
	tests := []struct {
		name    string
		template string
		wantErr bool
		errMsg  string
	}{
		{
			name:     "valid dotted properties",
			template: "Request {http.method} to {http.url} returned {http.status}",
			wantErr:  false,
		},
		{
			name:     "dotted property with format",
			template: "Duration: {http.duration.ms:F2}ms",
			wantErr:  false,
		},
		{
			name:     "dotted property with capturing",
			template: "User {@user.profile.data}",
			wantErr:  false,
		},
		{
			name:     "empty property name with dot",
			template: "Invalid {.}",
			wantErr:  true,
			errMsg:   "property name cannot be only dots",
		},
		{
			name:     "property starting with number",
			template: "Invalid {123.method}",
			wantErr:  true,
			errMsg:   "property name starts with a number",
		},
		{
			name:     "unclosed dotted property",
			template: "Unclosed {http.method",
			wantErr:  true,
			errMsg:   "unclosed property",
		},
		{
			name:     "dotted property with spaces",
			template: "Invalid {http. method}",
			wantErr:  true,
			errMsg:   "property name contains spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateTemplate() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}