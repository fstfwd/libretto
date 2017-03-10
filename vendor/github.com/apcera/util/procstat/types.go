// Copyright 2015 Apcera Inc. All rights reserved.

package procstat

import "time"

// ProcessStats contains information specific to the process that was queried.
type ProcessStats struct {
	// The amount of accumulated CPU time in nanoseconds.
	CpuNs time.Duration

	// The amount of resident set memory allocated to the process in bytes.
	RssBytes int64
}
