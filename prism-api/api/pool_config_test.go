package api

import "testing"

func TestComputeWorkerDefaultsUsesQueueAndCPU(t *testing.T) {
	tests := []struct {
		name        string
		queue       int
		cpu         int
		wantWorkers int
		wantBuffer  int
	}{
		{name: "fallbacks", queue: 0, cpu: 1, wantWorkers: 32, wantBuffer: 4096},
		{name: "queue scaled", queue: 32, cpu: 4, wantWorkers: 128, wantBuffer: 16384},
		{name: "cpu scaled", queue: 4, cpu: 8, wantWorkers: 192, wantBuffer: 24576},
		{name: "clamped upper", queue: 200, cpu: 32, wantWorkers: 192, wantBuffer: 24576},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workers, buffer := computeWorkerDefaults(tt.queue, tt.cpu)
			if workers != tt.wantWorkers {
				t.Fatalf("workers mismatch: got %d want %d", workers, tt.wantWorkers)
			}
			if buffer != tt.wantBuffer {
				t.Fatalf("buffer mismatch: got %d want %d", buffer, tt.wantBuffer)
			}
		})
	}
}
