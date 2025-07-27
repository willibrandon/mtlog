package disabled

type Logger struct{}
func (l *Logger) Information(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}
func (l *Logger) Debug(template string, args ...interface{}) {}
func (l *Logger) Error(template string, args ...interface{}) {}

func test() {
	log := &Logger{}
	// This would normally warn about lowercase property name but naming check is disabled
	log.Information("User {userId} logged in", 123)
	
	// This would normally suggest PascalCase
	log.Warning("Transaction {transaction_id} failed", "tx-123")
	
	// Other checks should still work
	log.Information("User {UserId} logged in from {IP}", 123) // want "template has 2 properties but 1 arguments provided"
}