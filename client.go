package googletrends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// HTTP header and content type constants used for API requests.
const (
	// headerKeyAccept is the HTTP header key for Accept content type.
	headerKeyAccept = "Accept"

	// headerKeyCookie is the HTTP header key for sending cookies.
	headerKeyCookie = "Cookie"

	// headerKeySetCookie is the HTTP header key for receiving cookies.
	headerKeySetCookie = "Set-Cookie"

	// headerKeyContentType is the HTTP header key for Content-Type.
	headerKeyContentType = "Content-Type"

	// headerKeyUserAgent is the HTTP header key for User-Agent.
	headerKeyUserAgent = "User-Agent"

	// contentTypeJSON is the MIME type for JSON content.
	contentTypeJSON = "application/json"

	// contentTypeForm is the MIME type for URL-encoded form data.
	contentTypeForm = "application/x-www-form-urlencoded;charset=UTF-8"

	// defaultUserAgent mimics a real browser to avoid rate limiting.
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"
)

// HTTPDoer is an interface for making HTTP requests.
// It abstracts the http.Client to allow for dependency injection and easier testing.
//
// The standard http.Client implements this interface, as does any custom
// client that provides a Do method with the same signature.
//
// Example custom implementation:
//
//	type loggingClient struct {
//	    client *http.Client
//	}
//
//	func (c *loggingClient) Do(req *http.Request) (*http.Response, error) {
//	    log.Printf("Request: %s %s", req.Method, req.URL)
//	    return c.client.Do(req)
//	}
type HTTPDoer interface {
	// Do sends an HTTP request and returns an HTTP response.
	// It follows redirects and handles cookies as configured.
	Do(req *http.Request) (*http.Response, error)
}

// gClient is the internal HTTP client for Google Trends API requests.
// It manages request defaults, caching, cookies for rate limiting, and debug mode.
//
// The client is thread-safe and uses read-write mutexes to protect cached data.
type gClient struct {
	// httpClient is the underlying HTTP client used for requests.
	// Defaults to http.DefaultClient but can be overridden with WithHTTPClient.
	httpClient HTTPDoer

	// defParams contains default query parameters applied to all requests.
	defParams url.Values

	// cm protects concurrent access to exploreCats.
	cm *sync.RWMutex

	// exploreCats caches the category tree to avoid repeated API calls.
	exploreCats *ExploreCatTree

	// lm protects concurrent access to exploreLocs.
	lm *sync.RWMutex

	// exploreLocs caches the location tree to avoid repeated API calls.
	exploreLocs *ExploreLocTree

	// cookie stores the session cookie received from rate-limited responses.
	// This cookie is automatically sent with subsequent requests to avoid further rate limiting.
	cookie string

	// debug enables verbose logging of requests and responses when true.
	debug bool
}

// Option is a functional option for configuring the gClient.
// Options are passed to newGClient to customize client behavior.
//
// The functional options pattern allows for clean, extensible configuration
// without breaking changes when new options are added.
type Option func(*gClient)

// WithHTTPClient returns an Option that sets a custom HTTP client.
// Use this to provide a client with custom timeouts, transport settings,
// or for testing with a mock client.
//
// Example:
//
//	customClient := &http.Client{
//	    Timeout: 30 * time.Second,
//	    Transport: &http.Transport{
//	        MaxIdleConns: 10,
//	    },
//	}
//	client := newGClient(WithHTTPClient(customClient))
func WithHTTPClient(httpClient HTTPDoer) Option {
	return func(c *gClient) {
		c.httpClient = httpClient
	}
}

// newGClient creates a new Google Trends client with default settings.
// It initializes the client with default parameters, mutexes for thread-safe
// caching, and applies any provided functional options.
//
// Options can be passed to customize the client behavior:
//
//	client := newGClient(WithHTTPClient(customHTTPClient))
//
// Without options, the client uses http.DefaultClient for HTTP requests.
func newGClient(opts ...Option) *gClient {
	p := make(url.Values)
	for k, v := range defaultParams {
		p.Add(k, v)
	}

	c := &gClient{
		httpClient: http.DefaultClient,
		defParams:  p,
		cm:         new(sync.RWMutex),
		lm:         new(sync.RWMutex),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// defaultParams returns a deep copy of the client's default URL parameters.
// This ensures that modifications to the returned map don't affect the original.
func (c *gClient) defaultParams() url.Values {
	out := make(map[string][]string, len(c.defParams))
	for i, v := range c.defParams {
		out[i] = make([]string, len(v))
		copy(out[i], v)
	}

	return out
}

// getCategories returns the cached category tree in a thread-safe manner.
// Returns nil if no categories have been cached yet.
func (c *gClient) getCategories() *ExploreCatTree {
	c.cm.RLock()
	defer c.cm.RUnlock()
	return c.exploreCats
}

// setCategories stores the category tree in the cache in a thread-safe manner.
func (c *gClient) setCategories(cats *ExploreCatTree) {
	c.cm.Lock()
	defer c.cm.Unlock()
	c.exploreCats = cats
}

// getLocations returns the cached location tree in a thread-safe manner.
// Returns nil if no locations have been cached yet.
func (c *gClient) getLocations() *ExploreLocTree {
	c.lm.RLock()
	defer c.lm.RUnlock()
	return c.exploreLocs
}

// setLocations stores the location tree in the cache in a thread-safe manner.
func (c *gClient) setLocations(locs *ExploreLocTree) {
	c.lm.Lock()
	defer c.lm.Unlock()
	c.exploreLocs = locs
}

// do performs an HTTP GET request to the specified URL.
// It handles rate limiting by extracting and reusing cookies from HTTP 429 responses.
//
// The method:
//   - Creates a new request with the provided context
//   - Adds Accept header for JSON content
//   - Includes any stored cookies from previous rate-limited responses
//   - Logs request/response details when debug mode is enabled
//   - Retries once with the cookie if rate limited (HTTP 429)
//
// Returns the response body as bytes or an error if the request fails.
func (c *gClient) do(ctx context.Context, u *url.URL) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCreateRequest, err)
	}

	r.Header.Add(headerKeyAccept, contentTypeJSON)
	r.Header.Add(headerKeyUserAgent, defaultUserAgent)

	if len(c.cookie) != 0 {
		r.Header.Add(headerKeyCookie, c.cookie)
	}

	if c.debug {
		log.Println("[Debug] Request with params: ", r.URL)
	}

	resp, err := c.httpClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errDoRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if c.debug {
		log.Println("[Debug] Response: ", resp)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		cookie := strings.Split(resp.Header.Get(headerKeySetCookie), ";")
		if len(cookie) > 0 {
			c.cookie = cookie[0]
			r.Header.Set(headerKeyCookie, cookie[0])

			resp, err = c.httpClient.Do(r)
			if err != nil {
				return nil, err
			}
			defer func() { _ = resp.Body.Close() }()
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: "+errReqDataF, ErrRequestFailed, resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// doPost performs an HTTP POST request to the specified URL with the given payload.
// It handles rate limiting by extracting and reusing cookies from HTTP 429 responses.
//
// The method:
//   - Creates a new POST request with the provided context and payload
//   - Sets Content-Type header to application/x-www-form-urlencoded
//   - Includes any stored cookies from previous rate-limited responses
//   - Logs request/response details when debug mode is enabled
//   - Retries once with the cookie if rate limited (HTTP 429)
//
// Returns the response body as bytes or an error if the request fails.
func (c *gClient) doPost(ctx context.Context, u *url.URL, payload string) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCreateRequest, err)
	}

	r.Header.Add(headerKeyContentType, contentTypeForm)
	r.Header.Add(headerKeyUserAgent, defaultUserAgent)

	if len(c.cookie) != 0 {
		r.Header.Add(headerKeyCookie, c.cookie)
	}

	if c.debug {
		log.Println("[Debug] POST Request with params: ", r.URL)
		log.Println("[Debug] POST Request payload: ", payload)
	}

	resp, err := c.httpClient.Do(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errDoRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if c.debug {
		log.Println("[Debug] Response: ", resp)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		cookie := strings.Split(resp.Header.Get(headerKeySetCookie), ";")
		if len(cookie) > 0 {
			c.cookie = cookie[0]
			r.Header.Set(headerKeyCookie, cookie[0])

			resp, err = c.httpClient.Do(r)
			if err != nil {
				return nil, err
			}
			defer func() { _ = resp.Body.Close() }()
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: "+errReqDataF, ErrRequestFailed, resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// unmarshal parses JSON string data into the destination struct.
// It wraps any JSON parsing errors with additional context.
func (c *gClient) unmarshal(str string, dest interface{}) error {
	if err := json.Unmarshal([]byte(str), dest); err != nil {
		return fmt.Errorf("%s: %w", errParsing, err)
	}

	return nil
}

// extractJSONFromResponse extracts trending search terms from the batch execute API response.
// The response format is a complex nested structure where the actual data is embedded
// as a JSON string within a JSON array.
//
// The method parses through each line of the response looking for JSON arrays,
// then extracts the trending topic strings from the nested structure.
//
// Returns a slice of trending search term strings or an error if parsing fails.
func (c *gClient) extractJSONFromResponse(text string) ([]string, error) {
	if c.debug {
		log.Println("[Debug] Extracting JSON from API response")
	}

	var result []string

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			parseErr := func() error {
				var intermediate []interface{}
				if err := json.Unmarshal([]byte(trimmed), &intermediate); err != nil {
					return err
				}

				if len(intermediate) > 2 {
					secondItem, ok := intermediate[0].([]interface{})
					if !ok || len(secondItem) < 3 {
						return errors.New("invalid intermediate format")
					}

					jsonStr, ok := secondItem[2].(string)
					if !ok {
						return errors.New("invalid json string format")
					}

					var data []interface{}
					if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
						return err
					}

					if len(data) > 1 {
						items, ok := data[1].([]interface{})
						if !ok {
							return errors.New("invalid items format")
						}

						for _, item := range items {
							if itemArr, ok := item.([]interface{}); ok && len(itemArr) > 0 {
								if topic, ok := itemArr[0].(string); ok {
									result = append(result, topic)
								}
							}
						}
					}
				}
				return nil
			}()

			if parseErr != nil {
				if c.debug {
					log.Println("[Debug] Error parsing JSON:", parseErr)
				}
				continue
			}

			if len(result) > 0 {
				if c.debug {
					log.Println("[Debug] JSON extraction successful")
				}
				return result, nil
			}
		}
	}

	return nil, errors.New("no valid JSON found in response")
}

// trendsNew fetches trending searches using the new Google Trends batch execute API.
// This method is used by DailyNew and DailyTrendingSearchNew functions.
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - hl: Host language code (e.g., "EN", "RU") - currently unused but kept for API consistency
//   - loc: Location code for regional trends (e.g., "US", "GB", "RU")
//
// Returns a slice of trending search term strings or an error if the request fails.
func (c *gClient) trendsNew(ctx context.Context, hl, loc string) ([]string, error) {
	u, _ := url.Parse(gBatchExecute)

	// Create payload for the new API
	payload := fmt.Sprintf("f.req=[[[i0OFE,\"[null, null, \\\"%s\\\", 0, null, 48]\"]]]", loc)

	if c.debug {
		log.Println("[Debug] Using new Google Trends API with payload:", payload)
	}

	data, err := c.doPost(ctx, u, payload)
	if err != nil {
		return nil, err
	}

	return c.extractJSONFromResponse(string(data))
}
