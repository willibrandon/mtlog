//go:build integration
// +build integration

package integration

import (
	"bytes"
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

func TestElasticsearchIntegration(t *testing.T) {
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	// Check if Elasticsearch is available
	if !isElasticsearchAvailable(esURL) {
		t.Skip("Elasticsearch is not available, skipping integration test")
	}

	// Create a unique test ID to identify our events
	testID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Create logger with Elasticsearch sink
	esSink, err := sinks.NewElasticsearchSink(esURL,
		sinks.WithElasticsearchIndex("mtlog-test"),
		sinks.WithElasticsearchBatchSize(10),
		sinks.WithElasticsearchBatchTimeout(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create Elasticsearch sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(esSink),
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
	err = fmt.Errorf("sample error for testing")
	log.Error("Error occurred in test {TestId}: {Error}", testID, err)

	// Force flush by closing the sink
	esSink.Close()

	// Wait for Elasticsearch to index the documents
	time.Sleep(5 * time.Second)

	// Query Elasticsearch to verify our events
	events, err := queryElasticsearchEvents(esURL, "mtlog-test-*", testID)
	if err != nil {
		t.Fatalf("Failed to query Elasticsearch events: %v", err)
	}
	
	// If no events found, try querying without the index pattern to see if events went elsewhere
	if len(events) == 0 {
		t.Logf("No events found in mtlog-test-*, checking all indices...")
		allEvents, _ := queryElasticsearchEvents(esURL, "*", testID)
		t.Logf("Found %d events in all indices", len(allEvents))
		for i, evt := range allEvents {
			t.Logf("Event %d index: %v", i, evt["_index"])
		}
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
		// Check required fields
		if _, ok := event["@timestamp"]; !ok {
			t.Error("Missing @timestamp field")
		}
		if _, ok := event["message"]; !ok {
			t.Error("Missing message field")
		}
		if _, ok := event["messageTemplate"]; !ok {
			t.Error("Missing messageTemplate field")
		}
		if level, ok := event["level"]; ok {
			levelCounts[level.(string)]++
		} else {
			t.Error("Missing level field")
		}

		// Check our test property
		if testRun, ok := event["TestRun"]; !ok || testRun != testID {
			t.Error("Missing or incorrect TestRun property")
		}

		// Check TestId property in events that have it
		if mt, ok := event["messageTemplate"].(string); ok && strings.Contains(mt, "{TestId}") {
			if eventTestID, ok := event["TestId"]; !ok || eventTestID != testID {
				t.Error("Missing or incorrect TestId property")
			}
		}

		// Check ECS fields
		if _, ok := event["log.level"]; !ok {
			t.Error("Missing log.level ECS field")
		}
		if _, ok := event["event.created"]; !ok {
			t.Error("Missing event.created ECS field")
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

	// Verify error field in error event
	errorEventFound := false
	for _, event := range events {
		if level, ok := event["level"]; ok && level == "Error" {
			if errorMsg, ok := event["error.message"]; !ok || errorMsg != "sample error for testing" {
				t.Error("Missing or incorrect error.message field in error event")
			}
			errorEventFound = true
			break
		}
	}
	if !errorEventFound {
		t.Error("Error event not found")
	}
}

func TestElasticsearchBatching(t *testing.T) {
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	if !isElasticsearchAvailable(esURL) {
		t.Skip("Elasticsearch is not available, skipping integration test")
	}

	testID := fmt.Sprintf("batch-test-%d", time.Now().UnixNano())

	// Create logger with small batch size to test batching
	esSink, err := sinks.NewElasticsearchSink(esURL,
		sinks.WithElasticsearchIndex("mtlog-batch-test"),
		sinks.WithElasticsearchBatchSize(5),
		sinks.WithElasticsearchBatchTimeout(10*time.Second), // Long timeout to ensure batch size triggers
	)
	if err != nil {
		t.Fatalf("Failed to create Elasticsearch sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(esSink),
		mtlog.WithProperty("BatchTest", testID),
	)

	// Log exactly 5 events to trigger a batch
	for i := 0; i < 5; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Wait for batch to be sent
	time.Sleep(2 * time.Second)

	// Query and verify
	events, err := queryElasticsearchEvents(esURL, "mtlog-batch-test-*", testID)
	if err != nil {
		t.Fatalf("Failed to query Elasticsearch events: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events from first batch, got %d", len(events))
	}

	// Log 3 more events (won't trigger batch by size)
	for i := 5; i < 8; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Close to force flush
	esSink.Close()

	time.Sleep(2 * time.Second)

	// Query again
	events, err = queryElasticsearchEvents(esURL, "mtlog-batch-test-*", testID)
	if err != nil {
		t.Fatalf("Failed to query Elasticsearch events: %v", err)
	}

	if len(events) != 8 {
		t.Errorf("Expected 8 total events, got %d", len(events))
	}
}

func TestElasticsearchDataStreams(t *testing.T) {
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	if !isElasticsearchAvailable(esURL) {
		t.Skip("Elasticsearch is not available, skipping integration test")
	}

	// Check if data streams are supported (ES 7.9+)
	version, err := getElasticsearchVersion(esURL)
	if err != nil || !supportsDataStreams(version) {
		t.Skip("Elasticsearch version does not support data streams")
	}

	// Create data stream template
	if err := createDataStreamTemplate(esURL, "mtlog-ds-test"); err != nil {
		t.Fatalf("Failed to create data stream template: %v", err)
	}

	testID := fmt.Sprintf("ds-test-%d", time.Now().UnixNano())

	// Create logger with data streams enabled
	esSink, err := sinks.NewElasticsearchSink(esURL,
		sinks.WithElasticsearchIndex("mtlog-ds-test"),
		sinks.WithElasticsearchDataStreams(),
		sinks.WithElasticsearchBatchSize(1),
	)
	if err != nil {
		t.Fatalf("Failed to create Elasticsearch sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(esSink),
		mtlog.WithProperty("DataStreamTest", testID),
	)

	log.Information("Data stream test: {TestId}", testID)

	// Close and wait
	esSink.Close()
	time.Sleep(2 * time.Second)

	// Verify event was logged to data stream
	events, err := queryElasticsearchEvents(esURL, "mtlog-ds-test", testID)
	if err != nil {
		t.Fatalf("Failed to query Elasticsearch events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Clean up data stream
	deleteDataStream(esURL, "mtlog-ds-test")
}

func TestElasticsearchAuthentication(t *testing.T) {
	esURL := os.Getenv("ELASTICSEARCH_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	if !isElasticsearchAvailable(esURL) {
		t.Skip("Elasticsearch is not available, skipping integration test")
	}

	// Test 1: Without authentication (should work with default test ES)
	t.Run("NoAuth", func(t *testing.T) {
		testID := fmt.Sprintf("noauth-test-%d", time.Now().UnixNano())

		esSink, err := sinks.NewElasticsearchSink(esURL,
			sinks.WithElasticsearchIndex("mtlog-auth-test"),
			sinks.WithElasticsearchBatchSize(1),
		)
		if err != nil {
			t.Fatalf("Failed to create Elasticsearch sink: %v", err)
		}

		log := mtlog.New(
			mtlog.WithSink(esSink),
			mtlog.WithProperty("AuthTest", testID),
		)

		log.Information("Test without authentication: {TestId}", testID)

		// Close and wait
		esSink.Close()
		time.Sleep(2 * time.Second)

		// Verify event was logged
		events, err := queryElasticsearchEvents(esURL, "mtlog-auth-test-*", testID)
		if err != nil {
			t.Fatalf("Failed to query Elasticsearch events: %v", err)
		}

		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
	})

	// Test 2: With authentication (if credentials are provided)
	apiKey := os.Getenv("ELASTICSEARCH_API_KEY")
	username := os.Getenv("ELASTICSEARCH_USERNAME")
	password := os.Getenv("ELASTICSEARCH_PASSWORD")

	if apiKey != "" || (username != "" && password != "") {
		t.Run("WithAuth", func(t *testing.T) {
			testID := fmt.Sprintf("auth-test-%d", time.Now().UnixNano())

			// Create sink with authentication
			var opts []sinks.ElasticsearchOption
			opts = append(opts, sinks.WithElasticsearchIndex("mtlog-auth-test"))
			opts = append(opts, sinks.WithElasticsearchBatchSize(1))

			if apiKey != "" {
				opts = append(opts, sinks.WithElasticsearchAPIKey(apiKey))
			} else {
				opts = append(opts, sinks.WithElasticsearchBasicAuth(username, password))
			}

			esSink, err := sinks.NewElasticsearchSink(esURL, opts...)
			if err != nil {
				t.Fatalf("Failed to create Elasticsearch sink: %v", err)
			}

			log := mtlog.New(
				mtlog.WithSink(esSink),
				mtlog.WithProperty("AuthTest", testID),
			)

			log.Information("Test with authentication: {TestId}", testID)

			// Close and wait
			esSink.Close()
			time.Sleep(2 * time.Second)

			// Verify event was logged
			events, err := queryElasticsearchEvents(esURL, "mtlog-auth-test-*", testID)
			if err != nil {
				t.Fatalf("Failed to query Elasticsearch events: %v", err)
			}

			if len(events) != 1 {
				t.Errorf("Expected 1 event, got %d", len(events))
			}
		})
	}
}

// Helper functions

func isElasticsearchAvailable(esURL string) bool {
	resp, err := http.Get(esURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func queryElasticsearchEvents(esURL, index, testID string) ([]map[string]interface{}, error) {
	// Refresh index to ensure documents are searchable
	refreshURL := fmt.Sprintf("%s/%s/_refresh", esURL, index)
	resp, err := http.Post(refreshURL, "application/json", nil)
	if err != nil {
		// Ignore refresh errors
	} else {
		resp.Body.Close()
	}

	// Build search query
	// Use .keyword field for exact matching on text fields
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"TestId.keyword": testID}},
					{"term": map[string]interface{}{"TestRun.keyword": testID}},
					{"term": map[string]interface{}{"BatchTest.keyword": testID}},
					{"term": map[string]interface{}{"DataStreamTest.keyword": testID}},
					{"term": map[string]interface{}{"AuthTest.keyword": testID}},
					// Also try without .keyword for fields that might be mapped as keyword
					{"term": map[string]interface{}{"TestId": testID}},
					{"term": map[string]interface{}{"TestRun": testID}},
					{"term": map[string]interface{}{"BatchTest": testID}},
					{"term": map[string]interface{}{"DataStreamTest": testID}},
					{"term": map[string]interface{}{"AuthTest": testID}},
				},
			},
		},
		"size": 1000,
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]string{"order": "asc"}},
		},
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	searchURL := fmt.Sprintf("%s/%s/_search", esURL, index)
	
	resp, err = http.Post(searchURL, "application/json", bytes.NewReader(queryJSON))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("elasticsearch query failed with status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	body, _ := io.ReadAll(resp.Body)
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	events := make([]map[string]interface{}, 0)
	for _, hit := range result.Hits.Hits {
		events = append(events, hit.Source)
	}

	return events, nil
}

func getElasticsearchVersion(esURL string) (string, error) {
	resp, err := http.Get(esURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}

	return info.Version.Number, nil
}

func supportsDataStreams(version string) bool {
	// Data streams are supported from 7.9+
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	
	major := parts[0]
	minor := parts[1]
	
	return (major == "7" && minor >= "9") || major >= "8"
}

func createDataStreamTemplate(esURL, dataStream string) error {
	template := map[string]interface{}{
		"index_patterns": []string{dataStream},
		"data_stream":    map[string]interface{}{},
		"template": map[string]interface{}{
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"@timestamp": map[string]interface{}{
						"type": "date",
					},
				},
			},
		},
	}

	templateJSON, err := json.Marshal(template)
	if err != nil {
		return err
	}

	templateURL := fmt.Sprintf("%s/_index_template/%s", esURL, dataStream)
	req, err := http.NewRequest("PUT", templateURL, bytes.NewReader(templateJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create template: %s", body)
	}

	return nil
}

func deleteDataStream(esURL, dataStream string) {
	deleteURL := fmt.Sprintf("%s/_data_stream/%s", esURL, dataStream)
	req, _ := http.NewRequest("DELETE", deleteURL, nil)
	http.DefaultClient.Do(req)

	// Also delete the template
	templateURL := fmt.Sprintf("%s/_index_template/%s", esURL, dataStream)
	req, _ = http.NewRequest("DELETE", templateURL, nil)
	http.DefaultClient.Do(req)
}