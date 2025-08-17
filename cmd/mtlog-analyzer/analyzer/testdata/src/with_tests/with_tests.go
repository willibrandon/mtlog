package with_tests

import "fmt"

// Simple logger type for testing With() method
type Logger struct{}

func (l *Logger) With(args ...interface{}) *Logger { return l }
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}

func testWithOddArguments() {
	log := &Logger{}
	
	// Valid cases - even number of arguments
	log.With()
	log.With("key1", "value1")
	log.With("key1", "value1", "key2", "value2")
	log.With("userId", 123, "requestId", "abc-123")
	
	// Invalid cases - odd number of arguments
	log.With("key1") // want `\[MTLOG009\] With\(\) requires an even number of arguments \(key-value pairs\), got 1`
	log.With("key1", "value1", "key2") // want `\[MTLOG009\] With\(\) requires an even number of arguments \(key-value pairs\), got 3`
	log.With("k1", "v1", "k2", "v2", "k3") // want `\[MTLOG009\] With\(\) requires an even number of arguments \(key-value pairs\), got 5`
}

func testWithNonStringKeys() {
	log := &Logger{}
	userId := 123
	var name string = "Alice"
	
	// Valid cases - string keys
	log.With("userId", 123)
	log.With("name", "Alice")
	log.With(name, "value")
	
	// Invalid cases - non-string keys
	log.With(123, "value") // want `\[MTLOG010\] With\(\) key must be a string, got numeric literal`
	log.With(3.14, "value") // want `\[MTLOG010\] With\(\) key must be a string, got float literal`
	log.With(userId, "value") // want `\[MTLOG010\] With\(\) key must be a string, got variable 'userId'`
	log.With(true, "value") // want `\[MTLOG010\] With\(\) key must be a string, got`
	
	// Mixed valid and invalid
	log.With("key1", "value1", 456, "value2") // want `\[MTLOG010\] With\(\) key must be a string, got numeric literal`
}

func testWithDuplicateKeys() {
	log := &Logger{}
	
	// Valid cases - unique keys
	log.With("key1", "value1", "key2", "value2", "key3", "value3")
	
	// Invalid cases - duplicate keys
	log.With("id", 1, "name", "test", "id", 2) // want `\[MTLOG003\] warning: duplicate key 'id' in With\(\) call`
	log.With("userId", 1, "userId", 2) // want `\[MTLOG003\] warning: duplicate key 'userId' in With\(\) call`
	log.With("a", 1, "b", 2, "c", 3, "a", 4) // want `\[MTLOG003\] warning: duplicate key 'a' in With\(\) call`
	
	// Multiple duplicates
	log.With("x", 1, "y", 2, "x", 3, "y", 4) // want `\[MTLOG003\] warning: duplicate key 'x' in With\(\) call` `\[MTLOG003\] warning: duplicate key 'y' in With\(\) call`
}

func testWithEmptyKeys() {
	log := &Logger{}
	
	// Valid cases - non-empty keys
	log.With("key", "value")
	log.With("a", 1, "b", 2)
	
	// Invalid cases - empty keys
	log.With("", "value") // want `\[MTLOG013\] With\(\) key is empty and will be ignored`
	log.With("key1", "value1", "", "ignored", "key2", "value2") // want `\[MTLOG013\] With\(\) key is empty and will be ignored`
	
	// Empty string variable (can't detect at compile time)
	emptyKey := ""
	log.With(emptyKey, "value") // No warning - can't determine value at compile time
}

func testWithReservedProperties() {
	log := &Logger{}
	
	// NOTE: Reserved property checking is OFF by default
	// These will NOT trigger warnings unless -check-reserved flag is enabled
	log.With("Timestamp", "custom")
	log.With("Level", "INFO")
	log.With("Message", "custom message")
	log.With("MessageTemplate", "template")
	log.With("Exception", fmt.Errorf("error"))
	log.With("SourceContext", "context")
	
	// Valid cases - not reserved
	log.With("UserId", 123)
	log.With("RequestId", "abc-123")
	log.With("CustomTimestamp", "2024-01-01")
}

func testWithComplexScenarios() {
	log := &Logger{}
	
	// Multiple issues in one call
	log.With(123, "value", "key", "value", "key") // want `\[MTLOG010\] With\(\) key must be a string, got numeric literal` `\[MTLOG009\] With\(\) requires an even number of arguments` `\[MTLOG003\] warning: duplicate key 'key' in With\(\) call`
	
	// Empty key with odd arguments
	log.With("", "value1", "key2") // want `\[MTLOG013\] With\(\) key is empty and will be ignored` `\[MTLOG009\] With\(\) requires an even number of arguments`
	
	// Non-string key with duplicate
	log.With("id", 1, 456, "value", "id", 2) // want `\[MTLOG010\] With\(\) key must be a string, got numeric literal` `\[MTLOG003\] warning: duplicate key 'id' in With\(\) call`
}

func testWithChaining() {
	log := &Logger{}
	
	// Chained With calls - each is analyzed separately
	log.With("key1", "value1").With("key2") // want `\[MTLOG009\] With\(\) requires an even number of arguments`
	log.With("id", 1).With("id", 2) // want `\[MTLOG011\] warning: With\(\) overrides property 'id' set in previous call`
	log.With(123, "value").With("key", "value") // want `\[MTLOG010\] With\(\) key must be a string, got numeric literal`
}

func testWithVariables() {
	log := &Logger{}
	
	// Variables as keys - type checking should still work
	const constKey = "constantKey"
	var varKey = "variableKey"
	var intKey = 123
	
	log.With(constKey, "value") // Valid - string constant
	log.With(varKey, "value") // Valid - string variable
	log.With(intKey, "value") // want `\[MTLOG010\] With\(\) key must be a string, got variable 'intKey'`
	
	// Interface{} type (edge case)
	var anyKey interface{} = "key"
	log.With(anyKey, "value") // want `\[MTLOG010\] With\(\) key must be a string`
}

func testWithExpressions() {
	log := &Logger{}
	
	// Function calls and expressions as keys
	log.With(fmt.Sprintf("key%d", 1), "value") // Valid - returns string
	log.With(len("test"), "value") // want `\[MTLOG010\] With\(\) key must be a string, got`
	
	// String concatenation
	log.With("prefix"+"suffix", "value") // Valid - results in string
}