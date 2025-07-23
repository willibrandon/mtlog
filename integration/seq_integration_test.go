//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestSeqIntegration(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	// Check if Seq is available
	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	// Create a unique test ID to identify our events
	testID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Create logger with Seq sink
	log := mtlog.New(
		mtlog.WithSeqAdvanced(seqURL,
			sinks.WithSeqBatchSize(10),
			sinks.WithSeqBatchTimeout(1*time.Second),
		),
		mtlog.WithMinimumLevel(core.VerboseLevel),
		mtlog.WithMachineName(),
		mtlog.WithProperty("TestRun", testID),
	)

	// Log various events
	log.Verbose("Verbose message {TestId}", testID)
	log.Debug("Debug message {TestId} with {Value}", testID, 42)
	log.Information("Information message {TestId}", testID)
	log.Warning("Warning message {TestId} with {Count}", testID, 5)

	// Log structured data
	order := struct {
		ID    string
		Total float64
		Items int
	}{
		ID:    "ORD-123",
		Total: 99.99,
		Items: 3,
	}
	log.Information("Order processed: {@Order} for test {TestId}", order, testID)

	// Log with error
	err := fmt.Errorf("sample error for testing")
	log.Error("Error occurred in test {TestId}: {Error}", testID, err)

	// Force flush by closing the logger
	log.Close()

	// Wait a bit for Seq to index the events
	time.Sleep(5 * time.Second)

	// Query Seq to verify our events
	events, err := querySeqEvents(seqURL, testID)
	if err != nil {
		t.Fatalf("Failed to query Seq events: %v", err)
	}

	// Verify we got the expected number of events
	if len(events) != 6 {
		t.Errorf("Expected 6 events, got %d", len(events))
		for i, evt := range events {
			t.Logf("Event %d: %+v", i, evt)
		}
	}

	// Verify event structure
	levelCounts := make(map[string]int)
	for _, event := range events {
		// Check required CLEF fields
		if _, ok := event["@t"]; !ok {
			t.Error("Missing @t (timestamp) field")
		}
		if _, ok := event["@mt"]; !ok {
			t.Error("Missing @mt (message template) field")
		}
		if level, ok := event["@l"]; ok {
			levelCounts[level.(string)]++
		} else {
			t.Error("Missing @l (level) field")
		}

		// Check our test property
		if testRun, ok := event["TestRun"]; !ok || testRun != testID {
			t.Error("Missing or incorrect TestRun property")
		}

		// Check TestId property in events that have it
		if mt, ok := event["@mt"].(string); ok && strings.Contains(mt, "{TestId}") {
			if eventTestID, ok := event["TestId"]; !ok || eventTestID != testID {
				t.Error("Missing or incorrect TestId property")
			}
		}
	}

	// Verify we got events at different levels
	expectedLevels := map[string]int{
		"Verbose":     1,
		"Debug":       1,
		"Information": 2,
		"Warning":     1,
		"Error":       1,
	}

	for level, expected := range expectedLevels {
		if actual := levelCounts[level]; actual != expected {
			t.Errorf("Expected %d %s events, got %d", expected, level, actual)
		}
	}
}

func TestSeqBatching(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	testID := fmt.Sprintf("batch-test-%d", time.Now().UnixNano())

	// Create logger with small batch size to test batching
	log := mtlog.New(
		mtlog.WithSeqAdvanced(seqURL,
			sinks.WithSeqBatchSize(5),
			sinks.WithSeqBatchTimeout(10*time.Second), // Long timeout to ensure batch size triggers
		),
		mtlog.WithProperty("BatchTest", testID),
	)

	// Log exactly 5 events to trigger a batch
	for i := 0; i < 5; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Wait for batch to be sent
	time.Sleep(2 * time.Second)

	// Query and verify
	events, err := querySeqEvents(seqURL, testID)
	if err != nil {
		t.Fatalf("Failed to query Seq events: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events from first batch, got %d", len(events))
	}

	// Log 3 more events (won't trigger batch by size)
	for i := 5; i < 8; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Close to force flush
	log.Close()

	time.Sleep(2 * time.Second)

	// Query again
	events, err = querySeqEvents(seqURL, testID)
	if err != nil {
		t.Fatalf("Failed to query Seq events: %v", err)
	}

	if len(events) != 8 {
		t.Errorf("Expected 8 total events, got %d", len(events))
	}
}

func TestSeqWithAPIKey(t *testing.T) {
	seqURL := os.Getenv("SEQ_URL")
	if seqURL == "" {
		seqURL = "http://localhost:5342"
	}

	apiKey := os.Getenv("SEQ_API_KEY")
	if apiKey == "" {
		t.Skip("SEQ_API_KEY not set, skipping API key test")
	}

	if !isSeqAvailable(seqURL) {
		t.Skip("Seq is not available, skipping integration test")
	}

	testID := fmt.Sprintf("apikey-test-%d", time.Now().UnixNano())

	// Create logger with API key
	log := mtlog.New(
		mtlog.WithSeqAPIKey(seqURL, apiKey),
		mtlog.WithProperty("APIKeyTest", testID),
	)

	log.Information("Test with API key: {TestId}", testID)

	// Close and wait
	log.Close()
	time.Sleep(2 * time.Second)

	// Verify event was logged
	events, err := querySeqEvents(seqURL, testID)
	if err != nil {
		t.Fatalf("Failed to query Seq events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}
}

// Helper functions

func isSeqAvailable(seqURL string) bool {
	// Test if we can post to the raw endpoint
	testEvent := `{"@t":"2025-01-22T00:00:00Z","@mt":"Health check","@l":"Information"}`
	resp, err := http.Post(seqURL+"/api/events/raw", "application/vnd.serilog.clef", strings.NewReader(testEvent))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func querySeqEvents(seqURL, testID string) ([]map[string]interface{}, error) {
	// Check if we have a separate query URL for CI environments
	queryBaseURL := os.Getenv("SEQ_QUERY_URL")
	if queryBaseURL == "" {
		// For local testing, we need to use port 8080 (web UI port) instead of 5341/5342 (ingestion port)
		queryBaseURL = strings.Replace(seqURL, ":5342", ":8080", 1)
		queryBaseURL = strings.Replace(queryBaseURL, ":5341", ":8080", 1)
	}
	queryURL := fmt.Sprintf("%s/api/events/signal?count=1000", queryBaseURL)

	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("seq query failed with status %d: %s\nURL: %s", resp.StatusCode, body, queryURL)
	}

	var result struct {
		Events []struct {
			Timestamp              string                   `json:"Timestamp"`
			Level                  string                   `json:"Level"`
			MessageTemplateTokens  []map[string]interface{} `json:"MessageTemplateTokens"`
			Properties             []struct {
				Name  string      `json:"Name"`
				Value interface{} `json:"Value"`
			} `json:"Properties"`
		} `json:"Events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	

	events := make([]map[string]interface{}, 0)
	for _, seqEvent := range result.Events {
		// Convert Seq event to our expected format
		event := map[string]interface{}{
			"@t": seqEvent.Timestamp,
			"@l": seqEvent.Level,
		}
		
		// Extract message template by concatenating all tokens
		var mt string
		for _, token := range seqEvent.MessageTemplateTokens {
			if text, ok := token["Text"].(string); ok {
				mt += text
			} else if propName, ok := token["PropertyName"].(string); ok {
				// This is a property placeholder
				mt += "{" + propName + "}"
			}
		}
		event["@mt"] = mt
		
		// Extract properties
		for _, prop := range seqEvent.Properties {
			event[prop.Name] = prop.Value
		}
		
		
		// Filter events based on testID
		match := false
		if val, ok := event["TestId"]; ok && val == testID {
			match = true
		} else if val, ok := event["TestRun"]; ok && val == testID {
			match = true
		} else if val, ok := event["BatchTest"]; ok && val == testID {
			match = true
		} else if val, ok := event["APIKeyTest"]; ok && val == testID {
			match = true
		}
		
		if match {
			events = append(events, event)
		}
	}

	return events, nil
}
