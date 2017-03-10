// Copyright 2013 Apcera, Inc. All rights reserved.

package hmac

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
)

// Compute the Hmac Sha1
// Takes a mesasge and secret as strings.
func ComputeHmacSha1(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha1.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
