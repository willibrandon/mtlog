//go:build integration
// +build integration

package integration

import (
	"os"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestSeqLevelControllerIntegration(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	// Check if Seq is available
	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	// Create level switch
	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)

	// Create Seq sink
	seqSink, err := sinks.NewSeqSink(seqURL)
	if err != nil {
		t.Fatalf("Failed to create Seq sink: %v", err)
	}
	defer seqSink.Close()

	// Test level controller options
	options := mtlog.SeqLevelControllerOptions{
		CheckInterval: 2 * time.Second,
		InitialCheck:  true,
	}

	// Create controller
	controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)
	defer controller.Close()

	// Test basic controller functionality
	currentLevel := controller.GetCurrentLevel()
	if currentLevel != core.InformationLevel {
		t.Errorf("Expected initial level Information, got %v", currentLevel)
	}

	// Test level switch change
	levelSwitch.SetLevel(core.WarningLevel)
	newLevel := controller.GetCurrentLevel()
	if newLevel != core.WarningLevel {
		t.Errorf("Expected level to change to Warning, got %v", newLevel)
	}

	t.Logf("Level controller basic functionality test passed")
}

func TestSeqLevelControllerForceCheck(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	levelSwitch := mtlog.NewLoggingLevelSwitch(core.DebugLevel)
	seqSink, err := sinks.NewSeqSink(seqURL)
	if err != nil {
		t.Fatalf("Failed to create Seq sink: %v", err)
	}
	defer seqSink.Close()

	options := mtlog.SeqLevelControllerOptions{
		CheckInterval: 1 * time.Hour, // Long interval
		InitialCheck:  false,
	}

	controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)
	defer controller.Close()

	// Test force check - this MUST work
	err = controller.ForceCheck()
	if err != nil {
		t.Fatalf("ForceCheck failed: %v", err)
	}

	// Basic controller functions should still work
	level := controller.GetCurrentLevel()
	t.Logf("Current level: %v", level)
}

func TestSeqLevelControllerConvenienceFunction(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	options := mtlog.SeqLevelControllerOptions{
		CheckInterval: 5 * time.Second,
		InitialCheck:  true,
	}

	// Test convenience function
	loggerOption, levelSwitch, controller := mtlog.WithSeqLevelControl(seqURL, options)
	defer controller.Close()

	// Create logger
	logger := mtlog.New(loggerOption)

	// Test that logger works
	logger.Information("Test message from Seq level controller integration")

	// Test level switch functionality
	levelSwitch.SetLevel(core.ErrorLevel)
	if levelSwitch.Level() != core.ErrorLevel {
		t.Errorf("Level switch should be Error, got %v", levelSwitch.Level())
	}

	logger.Information("This info message should be filtered")
	logger.Error("This error message should pass")

	logger.Close()
	t.Logf("Convenience function test completed")
}

func TestSeqLevelControllerBuilder(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	// Test builder pattern
	builder := mtlog.NewSeqLevelControllerBuilder(seqURL).
		WithCheckInterval(10 * time.Second).
		WithInitialCheck(true)

	// Test with custom level switch
	customSwitch := mtlog.NewLoggingLevelSwitch(core.WarningLevel)
	builder = builder.WithLevelSwitch(customSwitch)

	// Build
	loggerOption, levelSwitch, controller := builder.Build()
	defer controller.Close()

	// Should use the custom switch
	if levelSwitch != customSwitch {
		t.Error("Builder should use the provided level switch")
	}

	if levelSwitch.Level() != core.WarningLevel {
		t.Errorf("Expected Warning level, got %v", levelSwitch.Level())
	}

	// Create logger and test
	logger := mtlog.New(loggerOption)
	logger.Warning("Builder test message")
	logger.Close()

	t.Logf("Builder pattern test completed")
}

func TestSeqLevelControllerCleanup(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	levelSwitch := mtlog.NewLoggingLevelSwitch(core.InformationLevel)
	seqSink, err := sinks.NewSeqSink(seqURL)
	if err != nil {
		t.Fatalf("Failed to create Seq sink: %v", err)
	}
	defer seqSink.Close()

	options := mtlog.SeqLevelControllerOptions{
		CheckInterval: 100 * time.Millisecond,
		InitialCheck:  true,
	}

	controller := mtlog.NewSeqLevelController(levelSwitch, seqSink, options)

	// Let it run briefly
	time.Sleep(200 * time.Millisecond)

	// Close should be fast and clean
	start := time.Now()
	controller.Close()
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}

	t.Logf("Controller cleanup test passed, closed in %v", elapsed)
}

