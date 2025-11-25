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

const (
	headerKeyAccept      = "Accept"
	headerKeyCookie      = "Cookie"
	headerKeySetCookie   = "Set-Cookie"
	headerKeyContentType = "Content-Type"
	contentTypeJSON      = "application/json"
	contentTypeForm      = "application/x-www-form-urlencoded;charset=UTF-8"
)

// HTTPDoer is an interface for making HTTP requests.
// It allows for dependency injection and easier testing.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// gClient is the internal client for Google Trends API.
type gClient struct {
	httpClient HTTPDoer
	defParams  url.Values

	cm          *sync.RWMutex
	exploreCats *ExploreCatTree

	lm          *sync.RWMutex
	exploreLocs *ExploreLocTree

	cookie string
	debug  bool
}

// Option is a function that configures the client.
type Option func(*gClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient HTTPDoer) Option {
	return func(c *gClient) {
		c.httpClient = httpClient
	}
}

// newGClient creates a new Google Trends client with default settings.
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

func (c *gClient) defaultParams() url.Values {
	out := make(map[string][]string, len(c.defParams))
	for i, v := range c.defParams {
		out[i] = make([]string, len(v))
		copy(out[i], v)
	}

	return out
}

func (c *gClient) getCategories() *ExploreCatTree {
	c.cm.RLock()
	defer c.cm.RUnlock()
	return c.exploreCats
}

func (c *gClient) setCategories(cats *ExploreCatTree) {
	c.cm.Lock()
	defer c.cm.Unlock()
	c.exploreCats = cats
}

func (c *gClient) getLocations() *ExploreLocTree {
	c.lm.RLock()
	defer c.lm.RUnlock()
	return c.exploreLocs
}

func (c *gClient) setLocations(locs *ExploreLocTree) {
	c.lm.Lock()
	defer c.lm.Unlock()
	c.exploreLocs = locs
}

func (c *gClient) do(ctx context.Context, u *url.URL) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCreateRequest, err)
	}

	r.Header.Add(headerKeyAccept, contentTypeJSON)

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

// doPost performs a POST request to the specified URL with the given payload
func (c *gClient) doPost(ctx context.Context, u *url.URL, payload string) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCreateRequest, err)
	}

	r.Header.Add(headerKeyContentType, contentTypeForm)

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

func (c *gClient) unmarshal(str string, dest interface{}) error {
	if err := json.Unmarshal([]byte(str), dest); err != nil {
		return fmt.Errorf("%s: %w", errParsing, err)
	}

	return nil
}

// extractJSONFromResponse extracts the nested JSON object from the API response
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

// trendsNew uses the new Google Trends API to fetch trending searches
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
