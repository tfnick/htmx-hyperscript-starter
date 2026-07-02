package routes

import (
	"testing"

	"github.com/tfnick/go-svelte-starter/api/usecase"
)

func TestMonitorSampleResponseMapsProcessMemory(t *testing.T) {
	response := toMonitorSampleResponse(usecase.MonitorSampleCo{
		Memory: usecase.MonitorMemoryCo{
			UsedBytes:    100,
			TotalBytes:   200,
			UsagePercent: 50,
			Process: usecase.MonitorProcessMemoryCo{
				RSSBytes:       11,
				VMSBytes:       22,
				HeapAllocBytes: 33,
				HeapSysBytes:   44,
				SysBytes:       55,
				GCCount:        6,
			},
		},
	})

	if response.Memory.Process.RSSBytes != 11 ||
		response.Memory.Process.VMSBytes != 22 ||
		response.Memory.Process.HeapAllocBytes != 33 ||
		response.Memory.Process.HeapSysBytes != 44 ||
		response.Memory.Process.SysBytes != 55 ||
		response.Memory.Process.GCCount != 6 {
		t.Fatalf("process memory was not mapped: %#v", response.Memory.Process)
	}
}
