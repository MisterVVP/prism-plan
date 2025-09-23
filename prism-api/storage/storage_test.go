package storage

import (
	"encoding/base64"
	"testing"
)

func TestDecodeSettingsEntity(t *testing.T) {
	data := []byte(`{"PartitionKey":"u1","RowKey":"u1","TasksPerCategory":5,"ShowDoneTasks":true}`)
	s, err := decodeSettingsEntity(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.TasksPerCategory != 5 || !s.ShowDoneTasks {
		t.Fatalf("unexpected settings: %+v", s)
	}
}

func TestEncodeDecodeContinuationToken(t *testing.T) {
	pk := "p"
	rk := "r"
	token, err := encodeContinuationToken(&pk, &rk)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	dpk, drk, err := decodeContinuationToken(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dpk == nil || *dpk != pk {
		t.Fatalf("unexpected partition key: %v", dpk)
	}
	if drk == nil || *drk != rk {
		t.Fatalf("unexpected row key: %v", drk)
	}
}

func TestDecodeContinuationTokenInvalid(t *testing.T) {
	if _, _, err := decodeContinuationToken("not-base64"); err == nil {
		t.Fatal("expected error for malformed token")
	}
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"pk":""}`))
	if _, _, err := decodeContinuationToken(raw); err == nil {
		t.Fatal("expected error for missing components")
	}
}
