// Package httpx contains helper http functions
package httpx

import "net/http"

// IsRetryableStatus takes an http status code and returns whether or not it should
// be retryable in a temporal workflow
func IsRetryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	default:
		return code >= 500
	}
}
