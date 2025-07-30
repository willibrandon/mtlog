// Package dots tests the analyzer's handling of dotted property names
package dots

// Ensure we test dotted properties work correctly

// Logger mimics the mtlog logger interface for testing
type Logger struct{}

func (l *Logger) Verbose(template string, args ...interface{})     {}
func (l *Logger) V(template string, args ...interface{})           {}
func (l *Logger) Debug(template string, args ...interface{})       {}
func (l *Logger) D(template string, args ...interface{})           {}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Info(template string, args ...interface{})        {}
func (l *Logger) I(template string, args ...interface{})           {}
func (l *Logger) Warning(template string, args ...interface{})     {}
func (l *Logger) Warn(template string, args ...interface{})        {}
func (l *Logger) W(template string, args ...interface{})           {}
func (l *Logger) Error(template string, args ...interface{})       {}
func (l *Logger) Err(template string, args ...interface{})         {}
func (l *Logger) E(template string, args ...interface{})           {}
func (l *Logger) Fatal(template string, args ...interface{})       {}
func (l *Logger) F(template string, args ...interface{})           {}
func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }

func testAll() {
	log := &Logger{}
	
	// Valid OTEL-style properties
	log.Information("HTTP request {http.method} to {http.url} took {http.duration.ms}ms", "GET", "/api/users", 123.45)
	log.Information("Database {db.system} query to {db.name}", "postgres", "users")
	log.Information("Service {service.name} version {service.version}", "api", "1.0.0")
	
	// Dotted properties with format specifiers
	log.Information("Duration: {http.duration.ms:F2}ms", 123.456)
	log.Information("Status code: {http.status.code:000}", 200)
	
	// Dotted properties with destructuring
	userProfile := struct {
		Name  string
		Email string
	}{
		Name:  "John",
		Email: "john@example.com",
	}
	statusObj := 200
	log.Information("User profile: {@user.profile.data}", userProfile)
	log.Information("Status: {$http.response.status}", statusObj)
	
	// Mixed regular and dotted properties
	log.Information("User {UserId} made {http.method} request", 123, "POST")
	
	// Edge cases that should still work
	log.Information("Property {ends.with.dot.}", "value")
	log.Information("Property {has..consecutive..dots}", "value")
	
	// Should detect template/argument mismatch with dotted properties
	log.Information("Missing arg {http.method}") // want "template has 1 properties but 0 arguments provided"
	log.Information("Extra args {http.method}", "GET", "extra") // want "template has 1 properties but 2 arguments provided"
	
	// Should detect duplicate dotted properties
	log.Information("Duplicate {http.method} and {http.method}", "GET", "POST") // want "duplicate property 'http.method'"
	
	// OTEL-style dotted properties should NOT suggest PascalCase
	log.Information("User {user.id} logged in", 123) // OK - OTEL style
	
	// But non-dotted lowercase properties should still suggest PascalCase
	log.Information("User {userid} logged in", 123) // want "suggestion: consider using PascalCase for property 'userid'"
}