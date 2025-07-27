package commonkeys

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }

func test() {
	log := &Logger{}
	
	// Default common keys should be detected
	log.ForContext("user_id", 123).Information("User logged in") // want "suggestion: consider defining a constant for commonly used context key 'user_id'"
	log.ForContext("request_id", "req-456").Information("Processing request") // want "suggestion: consider defining a constant for commonly used context key 'request_id'"
	
	// Custom common keys added via flag
	log.ForContext("custom_id", "cust-789").Information("Custom action") // want "suggestion: consider defining a constant for commonly used context key 'custom_id'"
	log.ForContext("tenant_id", "tenant-001").Information("Tenant operation") // want "suggestion: consider defining a constant for commonly used context key 'tenant_id'"
	
	// Test case-insensitive matching
	log.ForContext("User_ID", 456).Information("Case variant 1") // want "suggestion: consider defining a constant for commonly used context key 'User_ID'"
	log.ForContext("USER_ID", 789).Information("Case variant 2") // want "suggestion: consider defining a constant for commonly used context key 'USER_ID'"
	log.ForContext("Request_Id", "req-789").Information("Case variant 3") // want "suggestion: consider defining a constant for commonly used context key 'Request_Id'"
	log.ForContext("REQUEST_ID", "req-999").Information("Case variant 4") // want "suggestion: consider defining a constant for commonly used context key 'REQUEST_ID'"
	log.ForContext("Custom_ID", "cust-111").Information("Custom case variant") // want "suggestion: consider defining a constant for commonly used context key 'Custom_ID'"
	log.ForContext("TENANT_ID", "tenant-222").Information("Tenant case variant") // want "suggestion: consider defining a constant for commonly used context key 'TENANT_ID'"
	
	// Non-common keys should not trigger suggestions
	log.ForContext("random_key", "value").Information("Random action")
	log.ForContext("RANDOM_KEY", "value").Information("Random action uppercase")
}

func testSeparators() {
	log := &Logger{}
	
	// Test various separators in common keys to ensure toPascalCase handles them
	log.ForContext("user.id", 123).Information("Dot separator") // want "suggestion: consider defining a constant for commonly used context key 'user.id'"
	log.ForContext("user-id", 456).Information("Hyphen separator") // want "suggestion: consider defining a constant for commonly used context key 'user-id'"
	log.ForContext("user:id", 789).Information("Colon separator") // want "suggestion: consider defining a constant for commonly used context key 'user:id'"
	log.ForContext("user/id", 999).Information("Slash separator") // want "suggestion: consider defining a constant for commonly used context key 'user/id'"
	
	// Test mixed separators
	log.ForContext("user_id.test-value", 111).Information("Mixed separators") // want "suggestion: consider defining a constant for commonly used context key 'user_id.test-value'"
	log.ForContext("request-id:trace_id", 222).Information("Multiple mixed") // want "suggestion: consider defining a constant for commonly used context key 'request-id:trace_id'"
}