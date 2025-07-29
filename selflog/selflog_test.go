package selflog_test

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/selflog"
)

func TestSelfLog(t *testing.T) {
	// Save original state
	selflog.Disable()
	defer selflog.Disable()

	t.Run("disabled by default", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Printf("[test] should not appear")
		if buf.Len() > 0 {
			t.Error("expected no output when disabled")
		}
	})

	t.Run("enable with writer", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Enable(&buf)
		defer selflog.Disable()

		selflog.Printf("[test] error: %s", "test error")
		
		output := buf.String()
		if !strings.Contains(output, "[test] error: test error") {
			t.Errorf("expected error message, got: %s", output)
		}
		if !strings.Contains(output, time.Now().UTC().Format("2006-01-02")) {
			t.Error("expected timestamp in output")
		}
	})

	t.Run("enable with func", func(t *testing.T) {
		var messages []string
		selflog.EnableFunc(func(msg string) {
			messages = append(messages, msg)
		})
		defer selflog.Disable()

		selflog.Printf("[sink] write failed: %v", "disk full")
		
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if !strings.Contains(messages[0], "[sink] write failed: disk full") {
			t.Errorf("unexpected message: %s", messages[0])
		}
	})

	t.Run("disable stops output", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Enable(&buf)
		selflog.Printf("[test] first")
		selflog.Disable()
		selflog.Printf("[test] second")
		
		output := buf.String()
		if strings.Contains(output, "second") {
			t.Error("expected no output after disable")
		}
	})

	t.Run("nil writer ignored", func(t *testing.T) {
		selflog.Enable(nil)
		selflog.Printf("[test] should not crash")
		// Should not panic
	})

	t.Run("nil func ignored", func(t *testing.T) {
		selflog.EnableFunc(nil)
		selflog.Printf("[test] should not crash")
		// Should not panic
	})
}

func TestSyncWriter(t *testing.T) {
	t.Run("concurrent writes", func(t *testing.T) {
		var unsafeBuf bytes.Buffer
		safeBuf := selflog.Sync(&unsafeBuf)
		
		selflog.Enable(safeBuf)
		defer selflog.Disable()

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				selflog.Printf("[goroutine-%d] test message", n)
			}(i)
		}
		wg.Wait()

		lines := strings.Split(strings.TrimSpace(unsafeBuf.String()), "\n")
		if len(lines) != 100 {
			t.Errorf("expected 100 lines, got %d", len(lines))
		}
	})
}

func TestRealWorldScenarios(t *testing.T) {
	t.Run("sink error", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Enable(&buf)
		defer selflog.Disable()

		// Simulate a sink reporting an error
		selflog.Printf("[file] write failed: %v (path=%s)", "permission denied", "/var/log/app.log")
		
		output := buf.String()
		if !strings.Contains(output, "permission denied") {
			t.Error("expected error details")
		}
		if !strings.Contains(output, "/var/log/app.log") {
			t.Error("expected path in output")
		}
	})

	t.Run("async sink panic", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Enable(&buf)
		defer selflog.Disable()

		// Simulate async sink panic recovery
		selflog.Printf("[async] worker panic: %v", "runtime error: index out of range")
		
		if !strings.Contains(buf.String(), "runtime error") {
			t.Error("expected panic details")
		}
	})

	t.Run("template parse error", func(t *testing.T) {
		var buf bytes.Buffer
		selflog.Enable(&buf)
		defer selflog.Disable()

		selflog.Printf("[parser] invalid template: %s at position %d", "unclosed property", 23)
		
		if !strings.Contains(buf.String(), "unclosed property at position 23") {
			t.Error("expected parse error details")
		}
	})
}

func BenchmarkSelfLog(b *testing.B) {
	b.Run("disabled", func(b *testing.B) {
		selflog.Disable()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			selflog.Printf("[bench] test message %d", i)
		}
	})

	b.Run("disabled with guard", func(b *testing.B) {
		selflog.Disable()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			if selflog.IsEnabled() {
				selflog.Printf("[bench] test message %d", i)
			}
		}
	})

	b.Run("enabled to discard", func(b *testing.B) {
		selflog.Enable(io.Discard)
		defer selflog.Disable()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			selflog.Printf("[bench] test message %d", i)
		}
	})

	b.Run("enabled with func", func(b *testing.B) {
		selflog.EnableFunc(func(string) {})
		defer selflog.Disable()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			selflog.Printf("[bench] test message %d", i)
		}
	})
}

// Race test
func TestRace(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race test in short mode")
	}

	var buf bytes.Buffer
	syncWriter := selflog.Sync(&buf)
	selflog.Enable(syncWriter)
	defer selflog.Disable()

	done := make(chan bool)
	
	// Multiple goroutines enabling/disabling
	go func() {
		for i := 0; i < 100; i++ {
			selflog.Enable(syncWriter)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			selflog.Disable()
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Multiple goroutines writing
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				selflog.Printf("[race-%d] message %d", n, j)
			}
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 12; i++ {
		<-done
	}
}