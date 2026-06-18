package trigger

import (
	"testing"
)

func TestVerifyHMAC_Valid(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	secret := "test-secret"
	sig := SignHMAC(secret, body)
	if !VerifyHMAC(secret, body, sig) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifyHMAC_Invalid(t *testing.T) {
	body := []byte(`{"event":"test"}`)
	if VerifyHMAC("wrong-secret", body, "deadbeef") {
		t.Fatal("expected invalid signature to fail")
	}
}

func TestNormalizeTodoistTaskCreated(t *testing.T) {
	payload := map[string]interface{}{
		"event_name": "item:added",
		"event_data": map[string]interface{}{
			"id":      "task-1",
			"content": "写本周周报",
		},
	}
	ev := NormalizeTodoist("item:added", "evt-123", payload)
	if ev.EventID != "evt-123" {
		t.Fatalf("event id: %s", ev.EventID)
	}
	if ev.EventType != "item:added" {
		t.Fatalf("event type: %s", ev.EventType)
	}
	if ev.Payload["content"] != "写本周周报" {
		t.Fatalf("payload not normalized: %v", ev.Payload)
	}
}
