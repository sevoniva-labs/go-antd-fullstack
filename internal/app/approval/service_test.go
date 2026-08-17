package approval

import "testing"

func TestCanonicalJSONIsStableAndPreservesLargeIntegers(t *testing.T) {
	left, err := canonicalJSON([]byte(`{"z":9007199254740993,"a":{"y":2,"x":1}}`))
	if err != nil {
		t.Fatalf("canonicalJSON(left) error = %v", err)
	}
	right, err := canonicalJSON([]byte(` { "a": {"x":1,"y":2}, "z":9007199254740993 } `))
	if err != nil {
		t.Fatalf("canonicalJSON(right) error = %v", err)
	}
	if string(left) != string(right) {
		t.Fatalf("canonical forms differ: %s != %s", left, right)
	}
	if string(left) != `{"a":{"x":1,"y":2},"z":9007199254740993}` {
		t.Fatalf("large integer changed: %s", left)
	}
}
