package analyzer

import "testing"

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"userId", "user_id"},
		{"UserID", "user_id"},
		{"userName", "user_name"},
		
		// Consecutive capitals
		{"HTTPServer", "http_server"},
		{"IOError", "io_error"},
		{"XMLParser", "xml_parser"},
		{"HTMLElement", "html_element"},
		{"URLPath", "url_path"},
		
		// All caps
		{"HTML", "html"},
		{"API", "api"},
		{"ID", "id"},
		{"HTTP", "http"},
		
		// Numbers
		{"userId123", "user_id_123"},
		{"user123Id", "user_123_id"},
		{"123userId", "123_user_id"},
		
		// Already snake_case
		{"already_snake_case", "already_snake_case"},
		{"snake_case_123", "snake_case_123"},
		{"UPPER_SNAKE_CASE", "upper_snake_case"},
		
		// Edge cases
		{"userID", "user_id"},
		{"getUserID", "get_user_id"},
		{"HTTPSConnection", "https_connection"},
		{"IPv4Address", "i_pv_4_address"},
		{"iOS", "i_os"},
		{"iPhone", "i_phone"},
		
		// Single character
		{"A", "a"},
		{"a", "a"},
		{"1", "1"},
		
		// Empty string
		{"", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}