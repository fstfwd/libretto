// Copyright 2015 Apcera Inc. All rights reserved.

package procstat

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

var (
	ticks    = int64(C.sysconf(C._SC_CLK_TCK))
	pagesize = int64(C.sysconf(C._SC_PAGESIZE))
)

// GetProcessStats will return the ProcessStats object associated with the given
// PID. If it fails to load the stats, then it will return an error.
func GetProcessStats(pid int) (*ProcessStats, error) {
	p := &ProcessStats{}

	// Load and Parse /proc/{pid}/stat to get all the data we need
	filename := fmt.Sprintf("/proc/%d/stat", pid)
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read process stats: %v", err)
	}
	fields := strings.Fields(string(b))
	// Magic number is the total fields in the file, as documented by proc.
	if len(fields) != 52 {
		return nil, fmt.Errorf("failed to parse proc stat file")
	}

	// Calculate the ticks for the process. Indexes based on the proc(5) manpage
	// for user/kernel ticks, as well as its children's user/kernel ticks.
	totalTicks, err := sum(fields[13], fields[14], fields[15], fields[16])
	if err != nil {
		return nil, fmt.Errorf("failed to calculate process CPU usage: %v", err)
	}

	// use float in going from ticks to time to ensure we preserve granularity
	// below 1 second.
	p.CpuNs = time.Duration(float64(totalTicks) / float64(ticks) * float64(time.Second))

	// Calculate the total resident set pages/size. Index based on man proc(5).
	totalPages, err := sum(fields[23])
	if err != nil {
		return nil, err
	}
	p.RssBytes = totalPages * pagesize

	return p, nil
}

// sum will convert the list of strings into int64s and total them together.
func sum(s ...string) (int64, error) {
	var total int64
	for _, v := range s {
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		total += i
	}
	return total, nil
}
