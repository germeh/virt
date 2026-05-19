package preflight

import (
	"fmt"

	"warm-migration-core/internal/domain"
)

type CutoverMode string

const CutoverWarm CutoverMode = "warm"

const (
	FailureCodeVMInvalid                       = "vm_invalid"
	FailureCodeCPUProfileUnsupported           = "cpu_profile_unsupported"
	FailureCodeMemoryCapacityInsufficient      = "memory_capacity_insufficient"
	FailureCodeStorageMappingMissing           = "storage_mapping_missing"
	FailureCodeStorageTargetMissing            = "storage_target_missing"
	FailureCodeStorageCapacityInsufficient     = "storage_capacity_insufficient"
	FailureCodeStorageDirtyTrackingUnsupported = "storage_dirty_tracking_unsupported"
	FailureCodeNetworkMappingMissing           = "network_mapping_missing"
	FailureCodeNetworkTargetMissing            = "network_target_missing"
	FailureCodeCutoverModeUnsupported          = "cutover_mode_unsupported"
)

type Request struct {
	VM          domain.VM
	Target      domain.ClusterProfile
	NetworkMap  map[string]string
	StorageMap  map[string]string
	CutoverMode CutoverMode
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Result struct {
	OK       bool      `json:"ok"`
	Failures []Failure `json:"failures"`
}

func Check(req Request) Result {
	var failures []Failure

	if err := req.VM.Validate(); err != nil {
		failures = append(failures, Failure{Code: FailureCodeVMInvalid, Message: err.Error()})
	}
	if req.CutoverMode != CutoverWarm {
		failures = append(failures, Failure{
			Code:    FailureCodeCutoverModeUnsupported,
			Message: fmt.Sprintf("cutover mode %q is unsupported; only %q is supported", req.CutoverMode, CutoverWarm),
		})
	}
	if !contains(req.Target.CPUProfiles, req.VM.CPUProfile) {
		failures = append(failures, Failure{
			Code:    FailureCodeCPUProfileUnsupported,
			Message: fmt.Sprintf("target cluster does not support CPU profile %q", req.VM.CPUProfile),
		})
	}
	if req.Target.AvailableMemoryBytes < req.VM.MemoryBytes {
		failures = append(failures, Failure{
			Code:    FailureCodeMemoryCapacityInsufficient,
			Message: "target cluster does not have enough available memory",
		})
	}

	requiredByStorageName := make(map[string]uint64)
	storageOverflow := make(map[string]bool)
	storageOrder := make([]string, 0)
	seenStorage := make(map[string]bool)
	for _, disk := range req.VM.Disks {
		targetStorageName := req.StorageMap[disk.StorageClass]
		if targetStorageName == "" {
			failures = append(failures, Failure{
				Code:    FailureCodeStorageMappingMissing,
				Message: fmt.Sprintf("source storage class %q has no target mapping", disk.StorageClass),
			})
			continue
		}
		targetStorage, ok := req.Target.StorageClasses[targetStorageName]
		if !ok {
			failures = append(failures, Failure{
				Code:    FailureCodeStorageTargetMissing,
				Message: fmt.Sprintf("target storage class %q does not exist", targetStorageName),
			})
			continue
		}

		if !seenStorage[targetStorageName] {
			seenStorage[targetStorageName] = true
			storageOrder = append(storageOrder, targetStorageName)
			if !targetStorage.SupportsDirtyTrack {
				failures = append(failures, Failure{
					Code:    FailureCodeStorageDirtyTrackingUnsupported,
					Message: fmt.Sprintf("target storage class %q does not support dirty tracking for warm migration", targetStorageName),
				})
			}
		}
		currentRequired := requiredByStorageName[targetStorageName]
		if currentRequired > ^uint64(0)-disk.VirtualSizeBytes {
			storageOverflow[targetStorageName] = true
			continue
		}
		requiredByStorageName[targetStorageName] = currentRequired + disk.VirtualSizeBytes
	}
	for _, targetStorageName := range storageOrder {
		targetStorage := req.Target.StorageClasses[targetStorageName]
		if storageOverflow[targetStorageName] || targetStorage.AvailableBytes < requiredByStorageName[targetStorageName] {
			failures = append(failures, Failure{
				Code:    FailureCodeStorageCapacityInsufficient,
				Message: fmt.Sprintf("target storage class %q cannot fit required storage", targetStorageName),
			})
		}
	}
	for _, nic := range req.VM.NICs {
		targetNetworkName := req.NetworkMap[nic.SourceNetwork]
		if targetNetworkName == "" {
			failures = append(failures, Failure{
				Code:    FailureCodeNetworkMappingMissing,
				Message: fmt.Sprintf("source network %q has no target mapping", nic.SourceNetwork),
			})
			continue
		}
		if _, ok := req.Target.Networks[targetNetworkName]; !ok {
			failures = append(failures, Failure{
				Code:    FailureCodeNetworkTargetMissing,
				Message: fmt.Sprintf("target network %q does not exist", targetNetworkName),
			})
		}
	}

	return Result{OK: len(failures) == 0, Failures: failures}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
