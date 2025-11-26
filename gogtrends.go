package googletrends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// client is the package-level Google Trends client instance.
// It is initialized with default settings and is safe for concurrent use.
var client = newGClient()

// Debug enables or disables debug logging for the Google Trends client.
// When enabled, request URLs, payloads, and response details are logged to stdout.
//
// This is useful for debugging API issues or understanding the request/response flow.
//
// Example:
//
//	googletrends.Debug(true)  // Enable debug logging
//	trends, _ := googletrends.Daily(ctx, "EN", "US")
//	googletrends.Debug(false) // Disable debug logging
func Debug(debug bool) {
	client.debug = debug
}

// Daily retrieves daily trending searches for a specific language and location.
// Results are ordered by date descending, with the most recent trends first.
//
// This function is an alias for DailyNew which uses the newer Google Trends API.
// For results grouped by day, use DailyTrendingSearchNew instead.
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - hl: Host language code (e.g., "EN", "RU", "ES")
//   - loc: Location code (e.g., "US", "GB", "RU")
//
// Returns a slice of TrendingSearch items or an error if the request fails.
//
// Example:
//
//	ctx := context.Background()
//	trends, err := googletrends.Daily(ctx, "EN", "US")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, trend := range trends {
//	    fmt.Println(trend.Title.Query)
//	}
func Daily(ctx context.Context, hl, loc string) ([]*TrendingSearch, error) {
	return DailyNew(ctx, hl, loc)
}

// ExploreCategories retrieves the complete tree of available Google Trends categories.
// The result is cached in the client for subsequent calls.
//
// Categories can be used with ExploreRequest.Category to filter trend results
// to specific topics like "Arts & Entertainment", "Business", "Technology", etc.
//
// Deprecated: This function uses the old Google Trends API which may be unstable.
// Consider using hardcoded category IDs for common categories instead.
//
// Example:
//
//	categories, err := googletrends.ExploreCategories(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Print top-level categories
//	for _, cat := range categories.Children {
//	    fmt.Printf("ID: %d, Name: %s\n", cat.ID, cat.Name)
//	}
func ExploreCategories(ctx context.Context) (*ExploreCatTree, error) {
	if cats := client.getCategories(); cats != nil {
		return cats, nil
	}

	u, _ := url.Parse(gAPI + gSCategories)

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(ExploreCatTree)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// cache in client
	client.setCategories(out)

	return out, nil
}

// ExploreLocations retrieves the complete tree of available geographic locations.
// The result is cached in the client for subsequent calls.
//
// Location codes can be used with ComparisonItem.Geo to filter trend results
// to specific countries, states, or regions.
//
// Deprecated: This function uses the old Google Trends API which may be unstable.
// Consider using standard ISO country/region codes instead (e.g., "US", "US-CA", "GB").
//
// Example:
//
//	locations, err := googletrends.ExploreLocations(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Print countries
//	for _, loc := range locations.Children {
//	    fmt.Printf("Code: %s, Name: %s\n", loc.ID, loc.Name)
//	}
func ExploreLocations(ctx context.Context) (*ExploreLocTree, error) {
	if locs := client.getLocations(); locs != nil {
		return locs, nil
	}

	u, _ := url.Parse(gAPI + gSGeo)

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(ExploreLocTree)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// cache in client
	client.setLocations(out)

	return out, nil
}

// Explore retrieves a list of widgets for the specified keywords and parameters.
// Each widget corresponds to a specific type of data visualization and contains
// the token required to fetch detailed data using other functions.
//
// Widget types returned:
//   - TIMESERIES: Use with InterestOverTime to get timeline chart data
//   - GEO_MAP: Use with InterestByLocation to get geographic distribution
//   - RELATED_QUERIES: Use with Related to get related search queries
//   - RELATED_TOPICS: Use with Related to get related topics
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - r: ExploreRequest containing keywords, time range, and filters
//   - hl: Host language code (e.g., "EN", "RU")
//
// Returns ExploreResponse (slice of widgets) or an error if the request fails.
//
// Example:
//
//	request := &googletrends.ExploreRequest{
//	    ComparisonItems: []*googletrends.ComparisonItem{
//	        {Keyword: "golang", Geo: "US", Time: "today 12-m"},
//	    },
//	    Category: 0,
//	    Property: "",
//	}
//	widgets, err := googletrends.Explore(ctx, request, "EN")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get timeline widget
//	timeWidgets := widgets.GetWidgetsByType(googletrends.IntOverTimeWidgetID)
//	timeline, _ := googletrends.InterestOverTime(ctx, timeWidgets[0], "EN")
func Explore(ctx context.Context, r *ExploreRequest, hl string) (ExploreResponse, error) {
	// hook for using incorrect `time` request (backward compatibility)
	for _, r := range r.ComparisonItems {
		r.Time = strings.ReplaceAll(r.Time, "+", " ")
	}

	u, _ := url.Parse(gAPI + gSExplore)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramHl, hl)

	// marshal request for query param
	reqBytes, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errInvalidRequest, err)
	}
	mReq := string(reqBytes)

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}'", "", 1)

	out := new(exploreOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.Widgets, nil
}

// InterestOverTime retrieves timeline data showing interest levels over the specified time period.
// The data is suitable for creating line charts showing how interest has changed over time.
//
// Each Timeline point contains:
//   - Timestamp information (Unix and formatted)
//   - Interest values (0-100) for each compared keyword
//   - Formatted values for display
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - w: An ExploreWidget of type TIMESERIES (obtained from Explore)
//   - hl: Host language code (e.g., "EN", "RU")
//
// Returns ErrInvalidWidgetType if the widget is not a TIMESERIES type.
//
// Example:
//
//	widgets, _ := googletrends.Explore(ctx, request, "EN")
//	timeWidgets := widgets.GetWidgetsByType(googletrends.IntOverTimeWidgetID)
//	timeline, err := googletrends.InterestOverTime(ctx, timeWidgets[0], "EN")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, point := range timeline {
//	    fmt.Printf("%s: %d\n", point.FormattedTime, point.Value[0])
//	}
func InterestOverTime(ctx context.Context, w *ExploreWidget, hl string) ([]*Timeline, error) {
	if !strings.HasPrefix(w.ID, string(IntOverTimeWidgetID)) {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSIntOverTime)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramHl, hl)
	p.Set(paramToken, w.Token)

	// Initialize empty Geo maps where needed
	for i, v := range w.Request.CompItem {
		if v != nil && len(v.Geo) == 0 {
			w.Request.CompItem[i].Geo = map[string]string{"": ""}
		}
	}

	// marshal request for query param
	reqBytes, err := json.Marshal(w.Request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errInvalidRequest, err)
	}
	mReq := string(reqBytes)

	p.Set(paramReq, mReq)
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(multilineOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.Default.TimelineData, nil
}

// InterestByLocation retrieves geographic distribution data showing interest by region.
// The data is suitable for creating choropleth maps showing regional interest levels.
//
// Each GeoMap entry contains:
//   - Geographic code and name
//   - Interest values (0-100) for each compared keyword
//   - Index of the keyword with highest interest in that region
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - w: An ExploreWidget of type GEO_MAP (obtained from Explore)
//   - hl: Host language code (e.g., "EN", "RU")
//
// Returns ErrInvalidWidgetType if the widget is not a GEO_MAP type.
//
// Example:
//
//	widgets, _ := googletrends.Explore(ctx, request, "EN")
//	geoWidgets := widgets.GetWidgetsByType(googletrends.IntOverRegionID)
//	regions, err := googletrends.InterestByLocation(ctx, geoWidgets[0], "EN")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, region := range regions {
//	    fmt.Printf("%s (%s): %d\n", region.GeoName, region.GeoCode, region.Value[0])
//	}
func InterestByLocation(ctx context.Context, w *ExploreWidget, hl string) ([]*GeoMap, error) {
	if !strings.HasPrefix(w.ID, string(IntOverRegionID)) {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSIntOverReg)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramHl, hl)
	p.Set(paramToken, w.Token)

	if len(w.Request.CompItem) > 1 {
		w.Request.DataMode = compareDataMode
	}

	// marshal request for query param
	reqBytes, err := json.Marshal(w.Request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errInvalidRequest, err)
	}

	p.Set(paramReq, string(reqBytes))
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(geoOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	return out.Default.GeoMapData, nil
}

// Related retrieves related topics or queries for a keyword.
// The function supports both RELATED_QUERIES and RELATED_TOPICS widget types.
//
// Each RankedKeyword contains:
//   - Query string (for queries) or Topic details (for topics)
//   - Relative interest value (0-100 or percentage increase)
//   - Link to explore the related term further
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - w: An ExploreWidget of type RELATED_QUERIES or RELATED_TOPICS (obtained from Explore)
//   - hl: Host language code (e.g., "EN", "RU")
//
// Returns ErrInvalidWidgetType if the widget is not a RELATED_QUERIES or RELATED_TOPICS type.
//
// Example:
//
//	widgets, _ := googletrends.Explore(ctx, request, "EN")
//
//	// Get related queries
//	queryWidgets := widgets.GetWidgetsByType(googletrends.RelatedQueriesID)
//	queries, _ := googletrends.Related(ctx, queryWidgets[0], "EN")
//	for _, q := range queries {
//	    fmt.Printf("%s: %s\n", q.Query, q.FormattedValue)
//	}
//
//	// Get related topics
//	topicWidgets := widgets.GetWidgetsByType(googletrends.RelatedTopicsID)
//	topics, _ := googletrends.Related(ctx, topicWidgets[0], "EN")
//	for _, t := range topics {
//	    fmt.Printf("%s (%s): %s\n", t.Topic.Title, t.Topic.Type, t.FormattedValue)
//	}
func Related(ctx context.Context, w *ExploreWidget, hl string) ([]*RankedKeyword, error) {
	if !strings.HasPrefix(w.ID, string(RelatedQueriesID)) && !strings.HasPrefix(w.ID, string(RelatedTopicsID)) {
		return nil, ErrInvalidWidgetType
	}

	u, _ := url.Parse(gAPI + gSRelated)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramHl, hl)
	p.Set(paramToken, w.Token)

	if len(w.Request.Restriction.Geo) == 0 {
		w.Request.Restriction.Geo[""] = ""
	}

	// marshal request for query param
	reqBytes, err := json.Marshal(w.Request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errInvalidRequest, err)
	}

	p.Set(paramReq, string(reqBytes))
	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(relatedOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// split all keywords together
	keywords := make([]*RankedKeyword, 0)
	for _, v := range out.Default.Ranked {
		keywords = append(keywords, v.Keywords...)
	}

	return keywords, nil
}

// Search provides autocomplete suggestions for a keyword query.
// Use this to find Google Knowledge Graph topics that match a search term,
// which can provide more precise results when used in ExploreRequest.
//
// Each KeywordTopic contains:
//   - Mid: Google Knowledge Graph machine ID for precise searching
//   - Title: Human-readable topic name
//   - Type: Category of the topic (e.g., "Programming language", "City")
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - word: The search term to get suggestions for
//   - hl: Host language code (e.g., "EN", "RU")
//
// Example:
//
//	suggestions, err := googletrends.Search(ctx, "python", "EN")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, s := range suggestions {
//	    fmt.Printf("%s (%s) - MID: %s\n", s.Title, s.Type, s.Mid)
//	}
//	// Output might include:
//	// Python (Programming language) - MID: /m/05z1_
//	// Python (Snake) - MID: /m/06blk
func Search(ctx context.Context, word, hl string) ([]*KeywordTopic, error) {
	req := fmt.Sprintf("%s%s/%s", gAPI, gSAutocomplete, url.QueryEscape(word))
	u, _ := url.Parse(req)

	p := make(url.Values)
	p.Set(paramTZ, "0")
	p.Set(paramHl, hl)

	u.RawQuery = p.Encode()

	b, err := client.do(ctx, u)
	if err != nil {
		return nil, err
	}

	// google api returns not valid json :(
	str := strings.Replace(string(b), ")]}',", "", 1)

	out := new(searchOut)
	if err := client.unmarshal(str, out); err != nil {
		return nil, err
	}

	// split all keywords together
	keywords := make([]*KeywordTopic, 0)
	keywords = append(keywords, out.Default.Keywords...)

	return keywords, nil
}

// DailyNew retrieves daily trending searches using the new Google Trends batch execute API.
// This is the recommended method for fetching daily trends as it uses a more stable API endpoint.
//
// Unlike DailyTrendingSearchNew, this function returns a flat list of trending searches
// without grouping by date.
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - hl: Host language code (e.g., "EN", "RU") - affects result language
//   - loc: Location code for regional trends (e.g., "US", "GB", "RU")
//
// Returns a slice of TrendingSearch items or an error if the request fails.
//
// Note: The new API returns only the search query; fields like FormattedTraffic,
// Image, and Articles will be empty.
//
// Example:
//
//	ctx := context.Background()
//	trends, err := googletrends.DailyNew(ctx, "EN", "US")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, trend := range trends {
//	    fmt.Println(trend.Title.Query)
//	}
func DailyNew(ctx context.Context, hl, loc string) ([]*TrendingSearch, error) {
	terms, err := client.trendsNew(ctx, hl, loc)
	if err != nil {
		return nil, err
	}

	searches := make([]*TrendingSearch, 0, len(terms))
	for _, term := range terms {
		searches = append(searches, &TrendingSearch{
			Title: &SearchTitle{
				Query: term,
			},
			FormattedTraffic: "",
			Image:            nil,
			Articles:         []*SearchArticle{},
		})
	}

	return searches, nil
}

// DailyTrendingSearchNew retrieves daily trending searches grouped by date using the new API.
// Results are returned as TrendingSearchDays, which groups trends by their date.
//
// This function is useful when you need to display trends organized by day,
// such as "Today's Trends", "Yesterday's Trends", etc.
//
// Parameters:
//   - ctx: Context for request cancellation and timeouts
//   - hl: Host language code (e.g., "EN", "RU") - affects result language
//   - loc: Location code for regional trends (e.g., "US", "GB", "RU")
//
// Returns a slice of TrendingSearchDays (currently only "Today") or an error.
//
// Note: The new API currently returns trends only for "Today". Historical data
// may not be available through this endpoint.
//
// Example:
//
//	ctx := context.Background()
//	days, err := googletrends.DailyTrendingSearchNew(ctx, "EN", "US")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, day := range days {
//	    fmt.Printf("=== %s ===\n", day.FormattedDate)
//	    for _, trend := range day.Searches {
//	        fmt.Println(trend.Title.Query)
//	    }
//	}
func DailyTrendingSearchNew(ctx context.Context, hl, loc string) ([]*TrendingSearchDays, error) {
	terms, err := client.trendsNew(ctx, hl, loc)
	if err != nil {
		return nil, err
	}

	today := &TrendingSearchDays{
		FormattedDate: "Today",
		Searches:      make([]*TrendingSearch, 0, len(terms)),
	}

	for _, term := range terms {
		today.Searches = append(today.Searches, &TrendingSearch{
			Title: &SearchTitle{
				Query: term,
			},
			FormattedTraffic: "",
			Image:            nil,
			Articles:         []*SearchArticle{},
		})
	}

	return []*TrendingSearchDays{today}, nil
}
