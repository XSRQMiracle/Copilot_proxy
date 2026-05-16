package proxy

import "testing"

func TestShouldExcludeFromStats(t *testing.T) {
	if !ShouldExcludeFromStats("/favicon.ico") {
		t.Fatal("expected favicon to be excluded")
	}
	if ShouldExcludeFromStats("/v1/chat/completions") {
		t.Fatal("expected API path to be counted")
	}
}
