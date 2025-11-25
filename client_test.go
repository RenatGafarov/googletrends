package googletrends

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient is a mock implementation of HTTPDoer for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

// newMockResponse creates a mock HTTP response with the given status code and body.
func newMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func TestNewGClient(t *testing.T) {
	t.Parallel()

	t.Run("creates client with default settings", func(t *testing.T) {
		c := newGClient()

		assert.NotNil(t, c)
		assert.NotNil(t, c.httpClient)
		assert.NotNil(t, c.defParams)
		assert.NotNil(t, c.cm)
		assert.NotNil(t, c.lm)
	})

	t.Run("creates client with custom HTTP client", func(t *testing.T) {
		mockClient := &mockHTTPClient{}
		c := newGClient(WithHTTPClient(mockClient))

		assert.Equal(t, mockClient, c.httpClient)
	})
}

func TestGClientDefaultParams(t *testing.T) {
	t.Parallel()

	c := newGClient()
	params := c.defaultParams()

	assert.NotNil(t, params)
	assert.Equal(t, "0", params.Get(paramTZ))
	assert.Equal(t, "EN", params.Get(paramHl))
}

func TestGClientDo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		mockResponse   *http.Response
		mockError      error
		expectedError  bool
		expectedResult string
	}{
		{
			name:           "successful request",
			mockResponse:   newMockResponse(http.StatusOK, `{"status": "ok"}`),
			mockError:      nil,
			expectedError:  false,
			expectedResult: `{"status": "ok"}`,
		},
		{
			name:          "request error",
			mockResponse:  nil,
			mockError:     errors.New("network error"),
			expectedError: true,
		},
		{
			name:          "non-200 status code",
			mockResponse:  newMockResponse(http.StatusInternalServerError, "error"),
			mockError:     nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					return tt.mockResponse, tt.mockError
				},
			}

			c := newGClient(WithHTTPClient(mockClient))
			u, _ := url.Parse("https://example.com/test")

			result, err := c.do(context.Background(), u)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, string(result))
			}
		})
	}
}

func TestGClientDoPost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		mockResponse   *http.Response
		mockError      error
		expectedError  bool
		expectedResult string
	}{
		{
			name:           "successful POST request",
			mockResponse:   newMockResponse(http.StatusOK, `{"result": "success"}`),
			mockError:      nil,
			expectedError:  false,
			expectedResult: `{"result": "success"}`,
		},
		{
			name:          "POST request error",
			mockResponse:  nil,
			mockError:     errors.New("network error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockHTTPClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, http.MethodPost, req.Method)
					return tt.mockResponse, tt.mockError
				},
			}

			c := newGClient(WithHTTPClient(mockClient))
			u, _ := url.Parse("https://example.com/test")

			result, err := c.doPost(context.Background(), u, "payload=test")

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, string(result))
			}
		})
	}
}

func TestGClientUnmarshal(t *testing.T) {
	t.Parallel()

	c := newGClient()

	t.Run("successful unmarshal", func(t *testing.T) {
		type testStruct struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		var result testStruct
		err := c.unmarshal(`{"name": "test", "value": 42}`, &result)

		assert.NoError(t, err)
		assert.Equal(t, "test", result.Name)
		assert.Equal(t, 42, result.Value)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var result map[string]interface{}
		err := c.unmarshal("invalid json", &result)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), errParsing)
	})
}

func TestGClientCategoriesCache(t *testing.T) {
	t.Parallel()

	c := newGClient()

	t.Run("get and set categories", func(t *testing.T) {
		assert.Nil(t, c.getCategories())

		cats := &ExploreCatTree{Name: "Test", ID: 1}
		c.setCategories(cats)

		result := c.getCategories()
		assert.NotNil(t, result)
		assert.Equal(t, "Test", result.Name)
		assert.Equal(t, 1, result.ID)
	})
}

func TestGClientLocationsCache(t *testing.T) {
	t.Parallel()

	c := newGClient()

	t.Run("get and set locations", func(t *testing.T) {
		assert.Nil(t, c.getLocations())

		locs := &ExploreLocTree{Name: "USA", ID: "US"}
		c.setLocations(locs)

		result := c.getLocations()
		assert.NotNil(t, result)
		assert.Equal(t, "USA", result.Name)
		assert.Equal(t, "US", result.ID)
	})
}

func TestGClientTooManyRequests(t *testing.T) {
	t.Parallel()

	callCount := 0
	mockClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				resp := newMockResponse(http.StatusTooManyRequests, "")
				resp.Header.Set(headerKeySetCookie, "test_cookie=value; Path=/")
				return resp, nil
			}
			return newMockResponse(http.StatusOK, `{"status": "ok"}`), nil
		},
	}

	c := newGClient(WithHTTPClient(mockClient))
	u, _ := url.Parse("https://example.com/test")

	result, err := c.do(context.Background(), u)

	require.NoError(t, err)
	assert.Equal(t, `{"status": "ok"}`, string(result))
	assert.Equal(t, 2, callCount)
	assert.Equal(t, "test_cookie=value", c.cookie)
}

func TestExtractJSONFromResponse(t *testing.T) {
	t.Parallel()

	c := newGClient()

	t.Run("no valid JSON returns error", func(t *testing.T) {
		result, err := c.extractJSONFromResponse("invalid response")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no valid JSON found in response")
	})

	t.Run("empty response returns error", func(t *testing.T) {
		result, err := c.extractJSONFromResponse("")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestDebugMode(t *testing.T) {
	c := newGClient()

	assert.False(t, c.debug)

	c.debug = true
	assert.True(t, c.debug)

	c.debug = false
	assert.False(t, c.debug)
}
