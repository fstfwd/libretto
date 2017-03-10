// Copyright 2015 Apcera Inc. All rights reserved.

package procstat

// GetProcessStats is a stub function on Darwin. This ensures it will compile
// and run without failure, but not report any process information.
func GetProcessStats(pid int) (*ProcessStats, error) {
	return &ProcessStats{}, nil
}
