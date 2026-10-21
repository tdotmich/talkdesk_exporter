package talkdesk

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// This is the event format from the Talkdesk stream documentation.
const exampleStream = "event:result\n" +
	`data:{"metadata":{"name":"total_arrived_contacts","timestamp":"2020-11-13T18:17:43.091587Z"},"result":[{"_value":7}]}` + "\n" +
	"\n" +
	"event:result\r\n" +
	`data:{"metadata":{"name":"abandon_rate","timestamp":"2020-11-13T18:17:48Z"},"result":[{"_value":"0.25"}]}` + "\r\n" +
	"\r\n"

func TestStreamReaderNext(t *testing.T) {
	streamReader := NewStreamReader(strings.NewReader(exampleStream))

	firstEvent, err := streamReader.Next()
	if err != nil {
		t.Fatalf("Next returned an unexpected error: %v", err)
	}

	if firstEvent.Type != "result" {
		t.Errorf("expected event type %q, got %q", "result", firstEvent.Type)
	}

	if firstEvent.Data.Metadata.Name != "total_arrived_contacts" {
		t.Errorf(
			"expected metadata name %q, got %q",
			"total_arrived_contacts",
			firstEvent.Data.Metadata.Name,
		)
	}

	value, err := firstEvent.Data.Value()
	if err != nil {
		t.Fatalf("Value returned an unexpected error: %v", err)
	}

	if value != 7 {
		t.Errorf("expected value 7, got %v", value)
	}

	secondEvent, err := streamReader.Next()
	if err != nil {
		t.Fatalf("Next returned an unexpected error: %v", err)
	}

	// The second event has Windows line endings and a value in a string.
	value, err = secondEvent.Data.Value()
	if err != nil {
		t.Fatalf("Value returned an unexpected error: %v", err)
	}

	if value != 0.25 {
		t.Errorf("expected value 0.25, got %v", value)
	}

	_, err = streamReader.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF at the end of the stream, got %v", err)
	}
}

func TestStreamReaderInvalidData(t *testing.T) {
	stream := "event:result\ndata:this is not json\n\n"

	streamReader := NewStreamReader(strings.NewReader(stream))

	_, err := streamReader.Next()
	if !errors.Is(err, ErrInvalidEventData) {
		t.Fatalf("expected ErrInvalidEventData, got %v", err)
	}
}

func TestStreamEventDataValueErrors(t *testing.T) {
	testCases := map[string]string{
		"empty list":    `[]`,
		"two rows":      `[{"_value":1},{"_value":2}]`,
		"null value":    `[{"_value":null}]`,
		"missing value": `[{"name":"agent"}]`,
		"not a list":    `"text"`,
	}

	for testName, result := range testCases {
		eventData := StreamEventData{Result: []byte(result)}

		_, err := eventData.Value()
		if err == nil {
			t.Errorf("%s: expected an error for result %s", testName, result)
		}
	}
}

func TestStreamEventDataRowCount(t *testing.T) {
	testCases := map[string]int{
		`[]`:             0,
		`[{"_value":7}]`: 1,
		`[{"contact_id":"c1"},{"contact_id":"c2"}]`: 2,
	}

	for result, expectedRows := range testCases {
		eventData := StreamEventData{Result: []byte(result)}

		rows, err := eventData.RowCount()
		if err != nil {
			t.Errorf("RowCount(%s) returned an unexpected error: %v", result, err)
		}

		if rows != expectedRows {
			t.Errorf("RowCount(%s): expected %d rows, got %d", result, expectedRows, rows)
		}
	}

	_, err := StreamEventData{Result: []byte(`"text"`)}.RowCount()
	if err == nil {
		t.Error("expected an error for a result that is not a list")
	}
}
