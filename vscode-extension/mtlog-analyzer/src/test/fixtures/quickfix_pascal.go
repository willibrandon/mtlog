package fixtures

import "github.com/willibrandon/mtlog"

func TestQuickFixPascal() {
	logger := mtlog.New(mtlog.WithConsole())
	logger.Information("User {user_id} logged in", 123)
}