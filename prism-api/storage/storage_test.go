package storage

import (
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
