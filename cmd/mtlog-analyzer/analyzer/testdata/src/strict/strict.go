package strict

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	// Unknown format should error in strict mode
	log.Information("Value: {Value:ZZZ}", 123) // want "invalid format specifier in property 'Value:ZZZ': unknown format specifier: ZZZ"
	
	// Valid formats should work
	log.Information("Count: {Count:000}", 42)
	log.Information("Price: {Price:F2}", 19.99)
}