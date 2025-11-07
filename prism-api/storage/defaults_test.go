package storage

import "testing"

func TestQueueConcurrencyForCPU(t *testing.T) {
	tests := []struct {
		name string
		cpu  int
		want int
	}{
		{name: "below minimum", cpu: 0, want: defaultQueueConcurrency},
		{name: "single cpu", cpu: 1, want: queuePerCPU},
		{name: "multi cpu scale", cpu: 4, want: 40},
		{name: "cap applied", cpu: 32, want: maxQueueConcurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := queueConcurrencyForCPU(tt.cpu)
			if got != tt.want {
				t.Fatalf("queueConcurrencyForCPU(%d) = %d, want %d", tt.cpu, got, tt.want)
			}
		})
	}
}
