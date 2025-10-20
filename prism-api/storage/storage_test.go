package storage

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDecodeSettingsEntity(t *testing.T) {
	testCases := map[string][]byte{
		"with_system_properties": []byte(`{"PartitionKey":"u1","RowKey":"u1","TasksPerCategory":5,"ShowDoneTasks":true}`),
		"metadata_removed":       []byte(`{"TasksPerCategory":5,"ShowDoneTasks":true}`),
	}
	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			s, err := decodeSettingsEntity(data)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if s.TasksPerCategory != 5 || !s.ShowDoneTasks {
				t.Fatalf("unexpected settings: %+v", s)
			}
		})
	}
}

func TestTaskEntityDecodeWithoutMetadata(t *testing.T) {
	payload := []byte(`{"RowKey":"task1","Title":"Write tests","Notes":"cover metadata removal","Category":"dev","Order":3,"Done":false}`)
	var ent taskEntity
	if err := json.Unmarshal(payload, &ent); err != nil {
		t.Fatalf("unmarshal task entity: %v", err)
	}
	if ent.RowKey != "task1" {
		t.Fatalf("unexpected row key: %s", ent.RowKey)
	}
	if ent.Title != "Write tests" {
		t.Fatalf("unexpected title: %s", ent.Title)
	}
	if ent.Notes != "cover metadata removal" {
		t.Fatalf("unexpected notes: %s", ent.Notes)
	}
	if ent.Category != "dev" {
		t.Fatalf("unexpected category: %s", ent.Category)
	}
	if ent.Order != 3 {
		t.Fatalf("unexpected order: %d", ent.Order)
	}
	if ent.Done {
		t.Fatalf("unexpected done value: %t", ent.Done)
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

func TestDecodeContinuationTokenLegacyJSON(t *testing.T) {
	pk := "legacy-pk"
	rk := "legacy-rk"
	legacy := continuationToken{PartitionKey: pk, RowKey: rk}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy token: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(data)
	dpk, drk, err := decodeContinuationToken(token)
	if err != nil {
		t.Fatalf("decode legacy token: %v", err)
	}
	if dpk == nil || *dpk != pk {
		t.Fatalf("unexpected partition key: %v", dpk)
	}
	if drk == nil || *drk != rk {
		t.Fatalf("unexpected row key: %v", drk)
	}
}

func TestResolveTaskPageSize(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		requested   int
		defaultSize int32
		want        int32
	}{
		{name: "use_default_when_missing", requested: 0, defaultSize: 30, want: 30},
		{name: "respect_smaller_request", requested: 10, defaultSize: 30, want: 10},
		{name: "clamp_to_max", requested: 5000, defaultSize: 30, want: maxTaskPageSize},
		{name: "sanitize_negative", requested: -10, defaultSize: 25, want: 25},
		{name: "sanitize_default_zero", requested: 0, defaultSize: 0, want: 1},
		{name: "clamp_large_default", requested: 0, defaultSize: 2000, want: maxTaskPageSize},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTaskPageSize(tc.requested, tc.defaultSize); got != tc.want {
				t.Fatalf("resolveTaskPageSize(%d, %d) = %d, want %d", tc.requested, tc.defaultSize, got, tc.want)
			}
		})
	}
}
