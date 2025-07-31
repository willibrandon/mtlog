package capture

import (
	"testing"
)

// User implements LogValue to provide a custom representation
type User struct {
	ID       int
	Username string
	Password string // Should not be logged
	Email    string
}

// LogValue returns a safe representation for logging
func (u User) LogValue() any {
	return map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"email":    maskEmail(u.Email),
		// Password is intentionally omitted
	}
}

func maskEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	// Show first character and domain
	atIndex := -1
	for i, ch := range email {
		if ch == '@' {
			atIndex = i
			break
		}
	}
	if atIndex > 0 && atIndex < len(email)-1 {
		return email[:1] + "***" + email[atIndex:]
	}
	return email[:1] + "***"
}

// CreditCard implements LogValue to safely log card info
type CreditCard struct {
	Number string
	Holder string
	CVV    string
}

func (c CreditCard) LogValue() any {
	// Only show last 4 digits
	masked := "****"
	if len(c.Number) >= 4 {
		masked = "**** **** **** " + c.Number[len(c.Number)-4:]
	}
	return struct {
		Number string
		Holder string
	}{
		Number: masked,
		Holder: c.Holder,
	}
}

// APIResponse implements LogValue for selective logging
type APIResponse struct {
	Status  int
	Body    []byte
	Headers map[string]string
}

func (r APIResponse) LogValue() any {
	// Don't log full body if it's large
	bodyPreview := string(r.Body)
	if len(r.Body) > 100 {
		bodyPreview = string(r.Body[:100]) + "..."
	}

	// Filter sensitive headers
	safeHeaders := make(map[string]string)
	for k, v := range r.Headers {
		if k != "Authorization" && k != "X-API-Key" {
			safeHeaders[k] = v
		}
	}

	return map[string]any{
		"status":  r.Status,
		"body":    bodyPreview,
		"headers": safeHeaders,
	}
}

func TestLogValue(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}

	t.Run("User with sensitive data", func(t *testing.T) {
		user := User{
			ID:       123,
			Username: "alice",
			Password: "secret123",
			Email:    "alice@example.com",
		}

		prop, ok := d.TryCapture(user, factory)
		if !ok {
			t.Fatal("TryCapture failed")
		}

		result, ok := prop.Value.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]interface{}, got %T", prop.Value)
		}

		// Check that password is not included
		if _, exists := result["password"]; exists {
			t.Error("Password should not be in log output")
		}

		// Check that email is masked
		if email, ok := result["email"].(string); !ok || email != "a***@example.com" {
			t.Errorf("Expected masked email 'a***@example.com', got %v", result["email"])
		}

		// Check other fields
		if result["id"] != 123 {
			t.Errorf("Expected id=123, got %v", result["id"])
		}
		if result["username"] != "alice" {
			t.Errorf("Expected username='alice', got %v", result["username"])
		}
	})

	t.Run("CreditCard masking", func(t *testing.T) {
		card := CreditCard{
			Number: "1234567812345678",
			Holder: "John Doe",
			CVV:    "123",
		}

		prop, _ := d.TryCapture(card, factory)

		// The result should be a struct (map after capturing)
		result, ok := prop.Value.(map[string]any)
		if !ok {
			t.Fatalf("Expected map[string]interface{}, got %T", prop.Value)
		}

		// Check masked number
		if number, ok := result["Number"].(string); !ok || number != "**** **** **** 5678" {
			t.Errorf("Expected masked number '**** **** **** 5678', got %v", result["Number"])
		}

		// CVV should not be present
		if _, exists := result["CVV"]; exists {
			t.Error("CVV should not be in log output")
		}
	})

	t.Run("APIResponse with filtering", func(t *testing.T) {
		resp := APIResponse{
			Status: 200,
			Body:   []byte(`{"data": "This is a very long response body that should be truncated in the logs for brevity and performance reasons"}`),
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer secret-token",
				"X-API-Key":     "secret-key",
				"X-Request-ID":  "abc123",
			},
		}

		prop, _ := d.TryCapture(resp, factory)
		result := prop.Value.(map[string]any)

		// Check body is truncated
		body := result["body"].(string)
		if len(body) > 103 { // 100 + "..."
			t.Errorf("Body should be truncated, got length %d", len(body))
		}

		// Check sensitive headers are filtered
		headers := result["headers"].(map[string]any)
		if _, exists := headers["Authorization"]; exists {
			t.Error("Authorization header should be filtered")
		}
		if _, exists := headers["X-API-Key"]; exists {
			t.Error("X-API-Key header should be filtered")
		}

		// Check safe headers are present
		if headers["Content-Type"] != "application/json" {
			t.Error("Content-Type header should be present")
		}
		if headers["X-Request-ID"] != "abc123" {
			t.Error("X-Request-ID header should be present")
		}
	})
}

func TestLogValueWithCachedCapturer(t *testing.T) {
	d := NewCachedCapturer()
	factory := &mockPropertyFactory{}

	// Test that LogValue works with cached capturer too
	user := User{
		ID:       456,
		Username: "bob",
		Password: "topsecret",
		Email:    "bob@test.com",
	}

	// Run twice to ensure caching doesn't interfere
	for i := 0; i < 2; i++ {
		prop, _ := d.TryCapture(user, factory)
		result := prop.Value.(map[string]any)

		if _, exists := result["password"]; exists {
			t.Errorf("Iteration %d: Password should not be in output", i)
		}

		if result["username"] != "bob" {
			t.Errorf("Iteration %d: Expected username='bob', got %v", i, result["username"])
		}
	}
}

// Nested LogValue test
type Team struct {
	Name    string
	Members []User
}

func (t Team) LogValue() any {
	return map[string]any{
		"name":        t.Name,
		"memberCount": len(t.Members),
		"members":     t.Members, // Will use User.LogValue for each member
	}
}

func TestNestedLogValue(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}

	team := Team{
		Name: "Development",
		Members: []User{
			{ID: 1, Username: "alice", Password: "pass1", Email: "alice@example.com"},
			{ID: 2, Username: "bob", Password: "pass2", Email: "bob@example.com"},
		},
	}

	prop, _ := d.TryCapture(team, factory)
	result := prop.Value.(map[string]any)

	if result["name"] != "Development" {
		t.Errorf("Expected team name='Development', got %v", result["name"])
	}

	if result["memberCount"] != 2 {
		t.Errorf("Expected memberCount=2, got %v", result["memberCount"])
	}

	// Check that members are captured using their LogValue
	members := result["members"].([]any)
	for i, member := range members {
		m := member.(map[string]any)
		if _, exists := m["password"]; exists {
			t.Errorf("Member %d: password should not be in output", i)
		}
		if _, exists := m["email"]; !exists {
			t.Errorf("Member %d: email should be in output", i)
		}
	}
}
