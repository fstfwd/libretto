// Copyright 2013 Apcera, Inc. All rights reserved.

package hmac

import (
	"testing"

	tt "github.com/apcera/util/testtool"
)

// Make sure that HMAC Sha1 is calculated correctly
func TestComputeHmacSha1(t *testing.T) {
	testHelper := tt.StartTest(t)
	defer testHelper.FinishTest()

	hmacSha1 := ComputeHmacSha1("message", "secret")
	tt.TestEqual(t, hmacSha1, "DK9kn+7klT2Hv5A6wRdsReAo3xY=")
}
