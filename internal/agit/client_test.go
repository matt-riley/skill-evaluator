package agit

import "testing"

func TestDecodeEnvelope(t *testing.T) {
	raw := []byte(`{"schema_version":"cli-json-v1","command":"log","data":{"origin":"pi","session_id":"s1","steps":[{"hash":"x"}]}}`)
	got, err := decodeEnvelope[Log](raw)
	if err != nil {
		t.Fatalf("decodeEnvelope error: %v", err)
	}
	if got.Origin != "pi" || got.SessionID != "s1" || len(got.Steps) != 1 || got.Steps[0].Hash != "x" {
		t.Errorf("decoded wrong: %+v", got)
	}
}
