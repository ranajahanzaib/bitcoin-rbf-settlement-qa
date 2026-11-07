package adapter

import (
	"testing"
)

func TestTargetValidation(t *testing.T) {
	for _, c := range []struct {
		url string
		ok  bool
	}{{"http://127.0.0.1:8080", true}, {"http://[::1]:8080", true}, {"http://localhost:8080", true}, {"http://example.org", false}, {"https://qa.example.org", true}, {"https://user:secret@example.org", false}, {"https://example.org?token=secret", false}, {"https://example.org/path", false}, {"file:///tmp/secret", false}} {
		_, err := NewHTTP(c.url, "")
		if (err == nil) != c.ok {
			t.Errorf("%s: %v", c.url, err)
		}
	}
}
