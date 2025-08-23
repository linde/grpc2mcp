package jsonrpc

import (
	"reflect"
	"testing"
)

// TODO make sure these tests match what JSONRpc can output and fail otw
func TestParseJsonRpcResponseBody(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		want    map[string]string
		wantErr bool
	}{
		{
			name: "valid input",
			body: []byte("key1: value1\nkey2: value2"),
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "empty input",
			body: []byte(""),
			want: map[string]string{},
		},
		{
			name: "malformed line",
			body: []byte("key1 value1"),
			want: map[string]string{},
		},
		{
			name: "extra whitespace",
			body: []byte("  key1  :  value1  "),
			want: map[string]string{"key1": "value1"},
		},
		{
			name: "empty lines",
			body: []byte("\nkey1: value1\n\nkey2: value2\n"),
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "multi-colon line",
			body: []byte("key1: value1:moredata"),
			want: map[string]string{"key1": "value1:moredata"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJsonRpcResponseBody(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJsonRpcResponseBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseJsonRpcResponseBody() = %v, want %v", got, tt.want)
			}
		})
	}
}
