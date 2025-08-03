package main

import "github.com/willibrandon/mtlog"

func main() {
    log := mtlog.New()
    log.Debug("Processing {user_id}", 456)
}