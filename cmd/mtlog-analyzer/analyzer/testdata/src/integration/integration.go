package integration

import "fmt"

// Simple logger type for testing without external dependencies
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
func (l *Logger) With(args ...interface{}) *Logger                 { return l }

func testTemplateArgMismatch() {
	log := &Logger{}
	userId := 123
	
	// Valid cases
	log.Information("User logged in")
	log.Information("User {UserId} logged in", userId)
	log.Debug("User {UserId} from {IP}", userId, "192.168.1.1")
	
	// Invalid cases
	log.Information("User {UserId} logged in from {IP}", userId) // want "template has 2 properties but 1 arguments provided"
	log.Warning("User {Name} with {Id} and {Email}")             // want "template has 3 properties but 0 arguments provided"
}

func testDuplicateProperties() {
	log := &Logger{}
	id := 123
	
	// Invalid: duplicate property
	log.Information("User {UserId} did {Action} as {UserId}", id, "login", id) // want "duplicate property 'UserId'"
	log.Debug("Value {Count} equals {Count:000}", 42, 42)                      // want "duplicate property 'Count'"
}

func testPropertyNaming() {
	log := &Logger{}
	
	// Invalid: spaces in property
	log.Information("User {User Id} logged in", 123) // want "property name 'User Id' contains spaces"
	
	// Invalid: starts with number
	log.Debug("Value {123Property}", "test") // want "property name '123Property' starts with a number"
	
	// Empty properties are filtered out by extractProperties, so no warning is generated
	log.Warning("User {} logged in")
	
	// Suggestion: lowercase
	log.Information("User {userId} logged in", 123) // want "suggestion: consider using PascalCase for property 'userId'"
}

func testCapturingUsage() {
	type User struct {
		ID   int
		Name string
	}
	
	log := &Logger{}
	user := User{ID: 1, Name: "Alice"}
	count := 42
	users := []User{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}}
	userMap := map[string]User{"alice": user}
	
	// Invalid: @ with basic type
	log.Information("Count is {@Count}", count) // want "warning: using @ prefix for basic type int, consider removing prefix"
	
	// Suggestion: complex type without @
	log.Debug("User {User} logged in", user) // want "suggestion: consider using @ prefix for complex type integration.User to enable capturing"
	
	// Valid: @ with complex type
	log.Information("User {@User} logged in", user)
	
	// Slices and maps should suggest @ prefix
	log.Information("Users: {Users}", users) // want "suggestion: consider using @ prefix for complex type \\[\\]integration.User to enable capturing"
	log.Information("User map: {UserMap}", userMap) // want "suggestion: consider using @ prefix for complex type map\\[string\\]integration.User to enable capturing"
}

func testErrorLogging() {
	log := &Logger{}
	
	// Invalid: Error level without error
	log.Error("Something went wrong") // want "suggestion: Error level log without error value, consider including the error or using Warning level"
	log.E("Failed to process")        // want "suggestion: Error level log without error value, consider including the error or using Warning level"
	
	// Valid: Error with actual error
	var err error
	log.Error("Failed to connect", err)
}

func testForContext() {
	log := &Logger{}
	
	// Suggestion for common keys
	log.ForContext("user_id", 123)         // want "suggestion: consider defining a constant for commonly used context key 'user_id'"
	log.ForContext("request_id", "abc-123") // want "suggestion: consider defining a constant for commonly used context key 'request_id'"
	
	// Valid: custom key
	log.ForContext("my_custom_key", "value")
}

func testFormatSpecifiers() {
	log := &Logger{}
	
	// Valid format specifiers
	log.Information("Count: {Count:000}", 42)
	log.Information("Price: {Price:F2}", 99.95)
	log.Information("Percentage: {Percent:P1}", 0.85)
	log.Information("Time: {Time:HH:mm:ss}", "14:30:00")
	
	// Currently we're lenient with unknown formats
	log.Information("Value: {Value:ZZZ}", 123)
}

func testShortMethods() {
	log := &Logger{}
	userId := 123
	
	// Short methods should also be checked
	log.V("Verbose {UserId} {Name}", userId)     // want "template has 2 properties but 1 arguments provided"
	log.D("Debug {Count} {Count}", 1, 1)         // want "duplicate property 'Count'"
	log.I("Info {user id}", userId)              // want "property name 'user id' contains spaces"
	log.W("Warning {}", "test")                  // want "template has 0 properties but 1 arguments provided"
	log.E("Error occurred")                      // want "suggestion: Error level log without error value, consider including the error or using Warning level"
	log.F("Fatal {UserId}", userId) // Valid
}

func testWithMethod() {
	log := &Logger{}
	userId := 123
	
	// Valid With() calls
	log.With("userId", 123)
	log.With("name", "Alice", "age", 30)
	
	// Invalid With() calls
	log.With("key1") // want `\[MTLOG009\] With\(\) requires an even number of arguments`
	log.With(userId, "value") // want `\[MTLOG010\] With\(\) key must be a string`
	log.With("id", 1, "id", 2) // want `\[MTLOG003\] warning: duplicate key 'id' in With\(\) call`
	log.With("", "value") // want `\[MTLOG013\] With\(\) key is empty and will be ignored`
}

func testEverything() {
	log := &Logger{}
	
	// Test multiple issues in one call
	log.Information("User {userId} {userId} logged in", 123, 456) // want "duplicate property 'userId' in template" "suggestion: consider using PascalCase for property 'userId'"
	
	// Test error logging
	log.Error("Failed to process") // want "suggestion: Error level log without error value, consider including the error or using Warning level"
	
	// Test with actual error
	err := fmt.Errorf("something went wrong")
	log.Error("Failed to process request", err) // Good - has error
	
	// Test capturing suggestions
	type User struct {
		Name string
		Age  int
	}
	user := User{Name: "Alice", Age: 30}
	log.Information("User logged in: {User}", user) // want "suggestion: consider using @ prefix for complex type integration.User to enable capturing"
	
	// Test format specifiers
	log.Information("Progress: {Percent:P2}", 0.456) // Good
	log.Information("Count: {Count:invalid}", 10)    // Good - lenient mode allows unknown formats
	
	// Test context usage
	log.ForContext("user_id", 123).Information("User action") // want "suggestion: consider defining a constant for commonly used context key 'user_id'"
	
	// Test extremely long templates
	log.Information("This is an extremely long template that contains many properties to test the analyzer's ability to handle very long templates without issues. It includes {Property1} and {Property2} and {Property3} and {Property4} and {Property5} and {Property6} and {Property7} and {Property8} and {Property9} and {Property10} and {Property11} and {Property12} and {Property13} and {Property14} and {Property15} and {Property16} and {Property17} and {Property18} and {Property19} and {Property20} and even more text to make this template really long and see if the analyzer can still properly extract all the properties and validate them correctly without any performance issues or bugs when dealing with such a long template string that goes on and on and on", 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20) // Good - matches all 20 properties
	
	// Test extremely long template with mismatch
	log.Information("Another long template with {A} and {B} and {C} and {D} and {E} and {F} and {G} and {H} and {I} and {J} and {K} and {L} and {M} and {N} and {O} and {P} and {Q} and {R} and {S} and {T} and {U} and {V} and {W} and {X} and {Y} and {Z} properties", 1, 2, 3) // want "template has 26 properties but 3 arguments provided"
}