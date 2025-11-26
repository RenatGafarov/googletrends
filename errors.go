package googletrends

import "errors"

// Internal error message constants used for error wrapping.
// These provide context when wrapping errors from underlying operations.
const (
	// errParsing is used when JSON parsing fails.
	errParsing = "failed to parse json"

	// errReqDataF is a format string for request failure details.
	// Arguments: HTTP status code (int), HTTP status text (string).
	errReqDataF = "request data: code = %d, status = %s"

	// errInvalidRequest is used when request parameters are invalid.
	errInvalidRequest = "invalid request param"

	// errCreateRequest is used when http.NewRequestWithContext fails.
	errCreateRequest = "failed to create request"

	// errDoRequest is used when the HTTP client fails to execute the request.
	errDoRequest = "failed to perform request"
)

// Sentinel errors for the Google Trends client.
// Use errors.Is() to check for these errors in your error handling code.
//
// Example:
//
//	widgets, err := googletrends.Explore(ctx, request, "EN")
//	if errors.Is(err, googletrends.ErrRequestFailed) {
//	    // Handle HTTP error (rate limiting, server error, etc.)
//	}
var (
	// ErrRequestFailed indicates that the HTTP request returned a non-200 status code.
	// This can occur due to rate limiting (HTTP 429), server errors (HTTP 5xx),
	// or invalid requests (HTTP 4xx).
	//
	// The error message includes the HTTP status code and status text for debugging.
	ErrRequestFailed = errors.New("failed to perform http request")

	// ErrInvalidWidgetType indicates that the provided widget is not compatible
	// with the called function.
	//
	// This error occurs when:
	//   - Calling InterestOverTime with a non-TIMESERIES widget
	//   - Calling InterestByLocation with a non-GEO_MAP widget
	//   - Calling Related with a widget that is neither RELATED_QUERIES nor RELATED_TOPICS
	//
	// Use ExploreResponse.GetWidgetsByType() to get the correct widget type.
	ErrInvalidWidgetType = errors.New("invalid widget type")
)
