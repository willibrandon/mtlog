package main

import (
    "errors"
    "net"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/core"
)

func main() {
    log := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithMinimumLevel(core.InformationLevel),
    )

    // Has err in scope from earlier
    conn, err := net.Dial("tcp", "localhost:8080")
    if err != nil {
        log.Error("Connection failed") // Should find 'err'
    }

    // Different block, no err
    if conn != nil {
        log.Error("Connection issue") // Should add 'nil' with TODO
    }

    // Ignore error with blank identifier
    _, _ = doSomething()
    log.Error("Ignored error case") // Should add 'nil' with TODO
}

func doSomething() (interface{}, error) {
    return nil, errors.New("test error")
}