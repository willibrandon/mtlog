module github.com/willibrandon/mtlog/adapters/sentry

go 1.21

require (
	github.com/getsentry/sentry-go v0.35.1
	github.com/willibrandon/mtlog v0.9.0
)

require (
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/willibrandon/mtlog => ../../
