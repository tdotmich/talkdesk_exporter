package talkdesk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidEventData means one event had bad data. The stream itself is still good,
// so the caller can skip the event and read the next one.
var ErrInvalidEventData = errors.New("invalid SSE event data")

type StreamEvent struct {
	Type string
	Data StreamEventData
}

// StreamEventData is the JSON in the "data:" line of an event. For example:
//
//	{"metadata":{"name":"total_arrived_contacts","timestamp":"2020-11-13T18:17:43.091587Z"},"result":[{"_value":0}]}
type StreamEventData struct {
	Metadata StreamEventMetaData `json:"metadata"`
	Result   json.RawMessage     `json:"result"`
}

type StreamEventMetaData struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
}

// resultRow is one row of the "result" list. We keep "_value" raw, because it can be
// a number (0) or a number in a string ("0.5").
type resultRow struct {
	Value json.RawMessage `json:"_value"`
}

type StreamReader struct {
	reader *bufio.Reader
}

// Read our new SSE connection
func NewStreamReader(stream io.Reader) *StreamReader {
	return &StreamReader{reader: bufio.NewReader(stream)}
}

// Next reads lines until a blank line ends an event, then returns that event.
// An SSE event looks like this:
//
//	event:result
//	data:{"metadata":{...},"result":[...]}
//	<blank line>
func (streamReader *StreamReader) Next() (*StreamEvent, error) {
	var eventType string
	var dataLines []string

	for {
		line, err := streamReader.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read SSE stream: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}

		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}

		// A blank line ends the event. If the event had no data, start again.
		if line == "" {
			if len(dataLines) > 0 {
				break
			}

			eventType = ""
		}

		if err == io.EOF {
			if len(dataLines) == 0 {
				return nil, io.EOF
			}
			break
		}
	}

	rawData := strings.Join(dataLines, "\n")

	var eventData StreamEventData

	err := json.Unmarshal([]byte(rawData), &eventData)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEventData, err)
	}

	return &StreamEvent{
		Type: eventType,
		Data: eventData,
	}, nil

}

// Value returns the number in the result. The exporter supports queries that return
// one row with one "_value", for example [{"_value":12}].
func (eventData StreamEventData) Value() (float64, error) {
	var rows []resultRow

	err := json.Unmarshal(eventData.Result, &rows)
	if err != nil {
		return 0, fmt.Errorf("result is not a list of rows: %w", err)
	}

	if len(rows) != 1 {
		return 0, fmt.Errorf("expected 1 result row, got %d", len(rows))
	}

	rawValue := strings.Trim(string(rows[0].Value), `"`)

	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return 0, fmt.Errorf("result row has no numeric _value: %q", rawValue)
	}

	return value, nil
}

// Time returns the time when Talkdesk calculated the result.
func (metadata StreamEventMetaData) Time() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, metadata.Timestamp)
}
