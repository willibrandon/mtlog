package string_to_const_test

// Simple logger for testing
type Logger struct{}

func (l *Logger) ForContext(key string, value interface{}) *Logger { return l }
func (l *Logger) Info(msg string) {}

func testMultipleOccurrences() {
	log := &Logger{}
	
	// Multiple occurrences of user_id - should offer comprehensive fix
	log.ForContext("user_id", 123).Info("First")
	log.ForContext("user_id", 456).Info("Second")
	log.ForContext("user_id", 789).Info("Third")
	
	// Multiple occurrences of request_id
	log.ForContext("request_id", "abc").Info("Request 1")
	log.ForContext("request_id", "def").Info("Request 2")
	
	// Single occurrence - should only offer simple replacement
	log.ForContext("trace_id", "xyz").Info("Trace")
}