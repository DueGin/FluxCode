package service

import (
	"fmt"
	"net/http"
)

// UpstreamHTTPError captures non-2xx upstream responses with status code, headers and body.
// It is mainly used by internal "upstream operations" (e.g. usage refresh) so callers can
// propagate the raw response and still allow RateLimitService to update account status.
type UpstreamHTTPError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (e *UpstreamHTTPError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("API returned status %d: %s", e.StatusCode, string(e.Body))
}

