package formatters

import (
	"encoding/json"
	"fmt"
	"strings"
	
	"github.com/willibrandon/mtlog/core"
)

// CLEFFormatter formats log events in Compact Log Event Format (CLEF) for Seq
type CLEFFormatter struct {
	// RenderMessage controls whether to render the message from the template
	RenderMessage bool
}

// NewCLEFFormatter creates a new CLEF formatter
func NewCLEFFormatter() *CLEFFormatter {
	return &CLEFFormatter{
		RenderMessage: true,
	}
}

// Format formats a log event in CLEF format
func (f *CLEFFormatter) Format(event *core.LogEvent) ([]byte, error) {
	clef := make(map[string]interface{})
	
	// Required CLEF fields
	clef["@t"] = event.Timestamp.UTC().Format("2006-01-02T15:04:05.0000000Z")
	clef["@mt"] = event.MessageTemplate
	clef["@l"] = levelToCLEF(event.Level)
	
	// Add rendered message if enabled
	if f.RenderMessage {
		if rendered, err := f.renderMessage(event); err == nil {
			clef["@m"] = rendered
		}
	}
	
	// Add properties
	for k, v := range event.Properties {
		// Skip properties that would conflict with CLEF reserved fields
		if !strings.HasPrefix(k, "@") {
			clef[k] = v
		}
	}
	
	// Add exception if present
	if err, ok := event.Properties["Exception"]; ok {
		if exception, ok := err.(error); ok {
			clef["@x"] = exception.Error()
		}
	}
	
	return json.Marshal(clef)
}

// renderMessage renders the message template with properties
func (f *CLEFFormatter) renderMessage(event *core.LogEvent) (string, error) {
	template := event.MessageTemplate
	result := strings.Builder{}
	
	// Simple rendering - replace {PropertyName} with values
	i := 0
	for i < len(template) {
		if i < len(template)-1 && template[i] == '{' {
			// Find closing brace
			j := i + 1
			for j < len(template) && template[j] != '}' {
				j++
			}
			
			if j < len(template) {
				// Extract property name
				propName := template[i+1 : j]
				
				// Remove destructuring hints
				propName = strings.TrimPrefix(propName, "@")
				propName = strings.TrimPrefix(propName, "$")
				
				// Look up property value
				if val, ok := event.Properties[propName]; ok {
					result.WriteString(fmt.Sprint(val))
				} else {
					// Keep the placeholder if no value found
					result.WriteString(template[i : j+1])
				}
				
				i = j + 1
				continue
			}
		}
		
		result.WriteByte(template[i])
		i++
	}
	
	return result.String(), nil
}

// levelToCLEF converts log level to CLEF format
func levelToCLEF(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "Verbose"
	case core.DebugLevel:
		return "Debug"
	case core.InformationLevel:
		return "Information"
	case core.WarningLevel:
		return "Warning"
	case core.ErrorLevel:
		return "Error"
	case core.FatalLevel:
		return "Fatal"
	default:
		return "Information"
	}
}

// CLEFBatchFormatter formats multiple events for batch submission
type CLEFBatchFormatter struct {
	formatter *CLEFFormatter
}

// NewCLEFBatchFormatter creates a new batch formatter
func NewCLEFBatchFormatter() *CLEFBatchFormatter {
	return &CLEFBatchFormatter{
		formatter: NewCLEFFormatter(),
	}
}

// FormatBatch formats multiple events as newline-delimited JSON
func (f *CLEFBatchFormatter) FormatBatch(events []*core.LogEvent) ([]byte, error) {
	var result []byte
	
	for _, event := range events {
		formatted, err := f.formatter.Format(event)
		if err != nil {
			return nil, err
		}
		
		result = append(result, formatted...)
		result = append(result, '\n')
	}
	
	return result, nil
}

// SeqIngestionFormat represents the format for Seq's ingestion API
type SeqIngestionFormat struct {
	Events []json.RawMessage `json:"events"`
}

// FormatForSeqIngestion formats events for Seq's ingestion API
func FormatForSeqIngestion(events []*core.LogEvent) ([]byte, error) {
	// Seq expects newline-delimited CLEF, not a JSON object
	batchFormatter := NewCLEFBatchFormatter()
	return batchFormatter.FormatBatch(events)
}