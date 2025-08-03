package fixtures

import "github.com/willibrandon/mtlog"

func TestQuickFixArgs() {
	logger := mtlog.New(mtlog.WithConsole())
	logger.Information("User {UserId} action {Action}", 123)
}