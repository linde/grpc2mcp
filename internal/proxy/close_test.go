package proxy

import (
	"testing"
)

func TestCloseLine(t *testing.T) {
	var result []string
	cl := &CloseLine{}
	cl.Add(func() {
		result = append(result, "a")
	})
	cl.Add(func() {
		result = append(result, "b")
	})
	cl.Add(func() {
		result = append(result, "c")
	})
	cl.Close()
	expected := []string{"a", "b", "c"}
	if len(result) != len(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("expected %v, got %v", expected, result)
		}
	}
}
