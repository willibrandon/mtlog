package main

import "github.com/willibrandon/mtlog"

func main() {
	log := mtlog.New(mtlog.WithConsole())

	// This should trigger an error - wrong number of args
	log.Information("User {UserId} logged in at {Time}", 123)

	// This should trigger a warning - non-PascalCase property
	log.Debug("Processing {user_id}", 456)

	// This is correct
	log.Warning("Disk usage at {Percentage:P1}", 0.85)
}
