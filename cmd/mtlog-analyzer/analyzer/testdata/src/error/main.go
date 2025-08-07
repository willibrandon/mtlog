package main

type Logger struct{}

func (l *Logger) Error(template string, args ...interface{}) {}
func (l *Logger) Warning(template string, args ...interface{}) {}

func main() {
	log := &Logger{}
	
	// This should produce MTLOG006
	log.Error("Something went wrong") // want `\[MTLOG006\] Error level log without error value`
	
	// This should NOT produce MTLOG006
	var err error
	log.Error("Operation failed", err)
	
	// This should NOT produce MTLOG006
	log.Warning("This is just a warning")
}