package mtlog007

import "github.com/willibrandon/mtlog"

func testContextKeyConstants() {
	log := mtlog.New()
	
	// Multiple occurrences of user_id - should create constant
	log.ForContext("user_id", 123).Information("First")     // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'user_id'`
	log.ForContext("user_id", 456).Information("Second")     // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'user_id'`
	log.ForContext("user_id", 789).Information("Third") // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'user_id'`
	
	// Multiple occurrences of request_id - should create constant  
	log.ForContext("request_id", "abc-123").Information("Request1")  // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'request_id'`
	log.ForContext("request_id", "def-456").Information("Request2") // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'request_id'`
	
	// Single occurrence of trace_id - should only suggest simple replacement
	log.ForContext("trace_id", "xyz-789").Information("Trace") // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'trace_id'`
	
	// Non-common key - should not trigger
	log.ForContext("my_custom_key", "value").Information("Custom")
}

func anotherFunction() {
	log := mtlog.New()
	
	// More occurrences of user_id in a different function
	log.ForContext("user_id", 111).Information("Another") // want `\[MTLOG007\] suggestion: consider defining a constant for commonly used context key 'user_id'`
}