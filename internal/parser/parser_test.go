package parser

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []MessageTemplateToken
		wantErr  bool
	}{
		{
			name:     "empty template",
			template: "",
			want:     []MessageTemplateToken{},
		},
		{
			name:     "text only",
			template: "Hello, World!",
			want: []MessageTemplateToken{
				&TextToken{Text: "Hello, World!"},
			},
		},
		{
			name:     "single property",
			template: "Hello, {Name}!",
			want: []MessageTemplateToken{
				&TextToken{Text: "Hello, "},
				&PropertyToken{PropertyName: "Name", Capturing: Default},
				&TextToken{Text: "!"},
			},
		},
		{
			name:     "multiple properties",
			template: "User {UserId} logged in from {IpAddress}",
			want: []MessageTemplateToken{
				&TextToken{Text: "User "},
				&PropertyToken{PropertyName: "UserId", Capturing: Default},
				&TextToken{Text: " logged in from "},
				&PropertyToken{PropertyName: "IpAddress", Capturing: Default},
			},
		},
		{
			name:     "capturing hints",
			template: "Processing {@User} with {$Exception}",
			want: []MessageTemplateToken{
				&TextToken{Text: "Processing "},
				&PropertyToken{PropertyName: "User", Capturing: Capture},
				&TextToken{Text: " with "},
				&PropertyToken{PropertyName: "Exception", Capturing: AsScalar},
			},
		},
		{
			name:     "escaped braces",
			template: "Use {{double braces}} to escape",
			want: []MessageTemplateToken{
				&TextToken{Text: "Use "},
				&TextToken{Text: "{"},
				&TextToken{Text: "double braces"},
				&TextToken{Text: "}"},
				&TextToken{Text: " to escape"},
			},
		},
		{
			name:     "unclosed property",
			template: "Hello {Name",
			want: []MessageTemplateToken{
				&TextToken{Text: "Hello "},
				&TextToken{Text: "{Name"},
			},
		},
		{
			name:     "empty property",
			template: "Hello {}!",
			want: []MessageTemplateToken{
				&TextToken{Text: "Hello "},
				&PropertyToken{PropertyName: "", Capturing: Default},
				&TextToken{Text: "!"},
			},
		},
		{
			name:     "property at start",
			template: "{Name} says hello",
			want: []MessageTemplateToken{
				&PropertyToken{PropertyName: "Name", Capturing: Default},
				&TextToken{Text: " says hello"},
			},
		},
		{
			name:     "property at end",
			template: "Hello, {Name}",
			want: []MessageTemplateToken{
				&TextToken{Text: "Hello, "},
				&PropertyToken{PropertyName: "Name", Capturing: Default},
			},
		},
		{
			name:     "adjacent properties",
			template: "{First}{Last}",
			want: []MessageTemplateToken{
				&PropertyToken{PropertyName: "First", Capturing: Default},
				&PropertyToken{PropertyName: "Last", Capturing: Default},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if len(got.Tokens) != len(tt.want) {
				t.Errorf("Parse() got %d tokens, want %d", len(got.Tokens), len(tt.want))
				return
			}
			
			for i, token := range got.Tokens {
				if !tokensEqual(token, tt.want[i]) {
					t.Errorf("Parse() token[%d] = %#v, want %#v", i, token, tt.want[i])
				}
			}
		})
	}
}

func TestExtractPropertyNames(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "no properties",
			template: "Hello, World!",
			want:     []string{},
		},
		{
			name:     "single property",
			template: "Hello, {Name}!",
			want:     []string{"Name"},
		},
		{
			name:     "multiple properties",
			template: "User {UserId} logged in from {IpAddress}",
			want:     []string{"UserId", "IpAddress"},
		},
		{
			name:     "duplicate properties",
			template: "{Name} is {Name}",
			want:     []string{"Name"},
		},
		{
			name:     "capturing hints",
			template: "Processing {@User} with {$Exception}",
			want:     []string{"User", "Exception"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPropertyNames(tt.template)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractPropertyNames() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidPropertyName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid simple", "Name", true},
		{"valid with underscore", "user_id", true},
		{"valid with number", "user123", true},
		{"starts with underscore", "_private", true},
		{"empty", "", false},
		{"starts with number", "123user", false},
		{"contains space", "user name", false},
		{"contains hyphen", "user-id", true}, // Allowed for Go compatibility
		{"contains dot", "user.id", true}, // Allowed for OTEL compatibility
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPropertyName(tt.input); got != tt.want {
				t.Errorf("isValidPropertyName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// tokensEqual compares two tokens for equality.
func tokensEqual(a, b MessageTemplateToken) bool {
	switch ta := a.(type) {
	case *TextToken:
		tb, ok := b.(*TextToken)
		return ok && ta.Text == tb.Text
	case *PropertyToken:
		tb, ok := b.(*PropertyToken)
		return ok && ta.PropertyName == tb.PropertyName && 
			ta.Capturing == tb.Capturing &&
			ta.Format == tb.Format &&
			ta.Alignment == tb.Alignment
	default:
		return false
	}
}