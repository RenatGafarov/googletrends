// Package googletrends provides an unofficial client library for the Google Trends API.
//
// This package allows you to programmatically access Google Trends data including:
//   - Daily trending searches by region
//   - Interest over time for specific keywords
//   - Interest by location (geographic distribution)
//   - Related topics and queries
//   - Keyword suggestions and autocomplete
//
// # Basic Usage
//
// To get daily trending searches:
//
//	ctx := context.Background()
//	trends, err := googletrends.Daily(ctx, "EN", "US")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, trend := range trends {
//	    fmt.Println(trend.Title.Query)
//	}
//
// To explore interest over time for a keyword:
//
//	request := &googletrends.ExploreRequest{
//	    ComparisonItems: []*googletrends.ComparisonItem{
//	        {Keyword: "golang", Time: "today 12-m"},
//	    },
//	    Category: 0,
//	    Property: "",
//	}
//	widgets, err := googletrends.Explore(ctx, request, "EN")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Rate Limiting
//
// The Google Trends API implements rate limiting. This client automatically handles
// HTTP 429 (Too Many Requests) responses by extracting and using cookies provided
// in the response for subsequent requests.
//
// # Thread Safety
//
// The package-level client is safe for concurrent use. Category and location caches
// are protected by read-write mutexes.
package googletrends

import (
	"sort"
	"strconv"
	"strings"
)

// API endpoint constants define the base URLs and paths for Google Trends API requests.
const (
	// gAPI is the base URL for the Google Trends API.
	gAPI = "https://trends.google.com/trends/api"

	// gSExplore is the endpoint path for exploring keywords and getting widgets.
	gSExplore = "/explore"

	// gSCategories is the endpoint path for retrieving available categories.
	gSCategories = "/explore/pickers/category"

	// gSGeo is the endpoint path for retrieving available geographic locations.
	gSGeo = "/explore/pickers/geo"

	// gSRelated is the endpoint path for fetching related searches data.
	gSRelated = "/widgetdata/relatedsearches"

	// gSIntOverTime is the endpoint path for interest over time (timeline) data.
	gSIntOverTime = "/widgetdata/multiline"

	// gSIntOverReg is the endpoint path for interest by region (geographic) data.
	gSIntOverReg = "/widgetdata/comparedgeo"

	// gSAutocomplete is the endpoint path for keyword autocomplete suggestions.
	gSAutocomplete = "/autocomplete"

	// gBatchExecute is the new API endpoint for batch execute requests.
	// This endpoint is used by the DailyNew and DailyTrendingSearchNew functions.
	gBatchExecute = "https://trends.google.com/_/TrendsUi/data/batchexecute"

	// paramHl is the query parameter key for host language (e.g., "EN", "RU").
	paramHl = "hl"

	// paramCat is the query parameter key for category filter.
	paramCat = "cat"

	// paramGeo is the query parameter key for geographic location filter.
	paramGeo = "geo"

	// paramReq is the query parameter key for the JSON-encoded request payload.
	paramReq = "req"

	// paramTZ is the query parameter key for timezone offset.
	paramTZ = "tz"

	// paramToken is the query parameter key for widget authentication token.
	paramToken = "token"

	// compareDataMode specifies the data mode for comparison requests.
	compareDataMode = "PERCENTAGES"
)

// WidgetType represents the type of a Google Trends widget.
// Each widget type corresponds to a specific visualization or data format
// returned by the Explore function.
type WidgetType string

// Widget type constants define the available widget types returned by the Explore function.
// These constants are used to filter widgets by type using ExploreResponse.GetWidgetsByType.
const (
	// IntOverTimeWidgetID identifies widgets containing interest over time data.
	// Use with InterestOverTime function to get timeline chart data.
	IntOverTimeWidgetID WidgetType = "TIMESERIES"

	// IntOverRegionID identifies widgets containing geographic interest data.
	// Use with InterestByLocation function to get regional map data.
	IntOverRegionID WidgetType = "GEO_MAP"

	// RelatedQueriesID identifies widgets containing related search queries.
	// Use with Related function to get related query suggestions.
	RelatedQueriesID WidgetType = "RELATED_QUERIES"

	// RelatedTopicsID identifies widgets containing related topics.
	// Use with Related function to get related topic suggestions.
	RelatedTopicsID WidgetType = "RELATED_TOPICS"
)

// defaultParams contains the default query parameters used for API requests.
// These values are copied and can be overridden for specific requests.
var defaultParams = map[string]string{
	paramTZ:  "0",      // Timezone offset (UTC)
	paramCat: "all",    // Category filter (all categories)
	"fi":     "0",      // First item index
	"fs":     "0",      // First sort index
	paramHl:  "EN",     // Host language (English)
	"ri":     "300",    // Results index
	"rs":     "20",     // Results size
}

// TrendingSearchDays represents a collection of trending searches grouped by date.
// This structure is returned by DailyTrendingSearchNew and organizes trends by day.
type TrendingSearchDays struct {
	// FormattedDate is a human-readable date string (e.g., "Today", "Yesterday", "Nov 25").
	FormattedDate string `json:"formattedDate" bson:"formatted_date"`

	// Searches contains the list of trending searches for this date.
	Searches []*TrendingSearch `json:"trendingSearches" bson:"searches"`
}

// TrendingSearch represents a single trending search item within a 24-hour period.
// It contains the search query, traffic information, associated image, and related news articles.
type TrendingSearch struct {
	// Title contains the search query information.
	Title *SearchTitle `json:"title" bson:"title"`

	// FormattedTraffic is a human-readable traffic string (e.g., "500K+", "1M+").
	FormattedTraffic string `json:"formattedTraffic" bson:"formatted_traffic"`

	// Image is an optional picture associated with the trending search.
	Image *SearchImage `json:"image" bson:"image"`

	// Articles contains news articles related to this trending search.
	Articles []*SearchArticle `json:"articles" bson:"articles"`
}

// SearchTitle represents the query string for a trending search.
type SearchTitle struct {
	// Query is the actual search term that is trending.
	Query string `json:"query" bson:"query"`
}

// SearchImage represents an image associated with a trending search or article.
type SearchImage struct {
	// NewsURL is the URL to the news article containing this image.
	NewsURL string `json:"newsUrl" bson:"news_url"`

	// Source is the name of the news source (e.g., "CNN", "BBC").
	Source string `json:"source" bson:"source"`

	// ImageURL is the direct URL to the image file.
	ImageURL string `json:"imageUrl" bson:"image_url"`
}

// SearchArticle represents a news article related to a trending search.
type SearchArticle struct {
	// Title is the headline of the news article.
	Title string `json:"title" bson:"title"`

	// TimeAgo is a relative time string (e.g., "2 hours ago", "1 day ago").
	TimeAgo string `json:"timeAgo" bson:"time_ago"`

	// Source is the name of the news publisher.
	Source string `json:"source" bson:"source"`

	// Image is an optional image associated with this article.
	Image *SearchImage `json:"image" bson:"image"`

	// URL is the link to the full article.
	URL string `json:"url" bson:"url"`

	// Snippet is a brief excerpt or summary of the article content.
	Snippet string `json:"snippet" bson:"snippet"`
}


// ExploreRequest is the input structure for the Explore function.
// It can contain multiple comparison items (keywords) to discover and compare trends.
//
// Example usage:
//
//	request := &ExploreRequest{
//	    ComparisonItems: []*ComparisonItem{
//	        {Keyword: "golang", Geo: "US", Time: "today 12-m"},
//	        {Keyword: "python", Geo: "US", Time: "today 12-m"},
//	    },
//	    Category: 0,  // All categories
//	    Property: "", // Web search (empty) or "youtube", "news", "froogle"
//	}
type ExploreRequest struct {
	// ComparisonItems contains one or more keywords to explore and compare.
	// Maximum of 5 comparison items are typically supported.
	ComparisonItems []*ComparisonItem `json:"comparisonItem" bson:"comparison_items"`

	// Category filters results to a specific category.
	// Use 0 for all categories, or a specific category ID from ExploreCategories.
	Category int `json:"category" bson:"category"`

	// Property specifies the Google property to search.
	// Valid values: "" (web search), "youtube", "news", "froogle" (shopping), "images".
	Property string `json:"property" bson:"property"`
}

// ComparisonItem represents a single keyword for comparison in an ExploreRequest.
// It includes the search term, geographic filter, and time range parameters.
//
// Time format examples:
//   - "now 1-H" - past hour
//   - "now 4-H" - past 4 hours
//   - "now 1-d" - past day
//   - "now 7-d" - past 7 days
//   - "today 1-m" - past month
//   - "today 3-m" - past 3 months
//   - "today 12-m" - past 12 months
//   - "today 5-y" - past 5 years
//   - "all" - all available data (2004 to present)
//   - "2020-01-01 2020-12-31" - custom date range
type ComparisonItem struct {
	// Keyword is the search term to analyze.
	Keyword string `json:"keyword" bson:"keyword"`

	// Geo is the geographic location code (e.g., "US", "GB", "RU").
	// Leave empty for worldwide data. Use ExploreLocations to get valid codes.
	Geo string `json:"geo,omitempty" bson:"geo"`

	// Time specifies the time range for the data.
	// See type documentation for format examples.
	Time string `json:"time" bson:"time"`

	// GranularTimeResolution enables more granular time data when true.
	// Useful for short time ranges to get hourly instead of daily data.
	GranularTimeResolution bool `json:"granularTimeResolution" bson:"granular_time_resolution"`

	// StartTime is an alternative way to specify the start of the time range.
	// Format: Unix timestamp in seconds.
	StartTime string `json:"startTime" bson:"start_time"`

	// EndTime is an alternative way to specify the end of the time range.
	// Format: Unix timestamp in seconds.
	EndTime string `json:"endTime" bson:"end_time"`
}

// ExploreCatTree represents a hierarchical tree of Google Trends categories.
// Each node can contain child categories, forming a complete category taxonomy.
//
// Deprecated: This type is used by the deprecated ExploreCategories function.
type ExploreCatTree struct {
	// Name is the human-readable category name (e.g., "Arts & Entertainment").
	Name string `json:"name" bson:"name"`

	// ID is the unique numeric identifier for this category.
	// Use this ID in ExploreRequest.Category to filter by category.
	ID int `json:"id" bson:"id"`

	// Children contains subcategories of this category.
	Children []*ExploreCatTree `json:"children" bson:"children"`
}

// ExploreLocTree represents a hierarchical tree of geographic locations.
// Each node can contain child locations (e.g., Country -> State -> City).
//
// Deprecated: This type is used by the deprecated ExploreLocations function.
type ExploreLocTree struct {
	// Name is the human-readable location name (e.g., "United States", "California").
	Name string `json:"name" bson:"name"`

	// ID is the geographic code for this location (e.g., "US", "US-CA").
	// Use this ID in ComparisonItem.Geo to filter by location.
	ID string `json:"id" bson:"id"`

	// Children contains sub-locations within this geographic area.
	Children []*ExploreLocTree `json:"children" bson:"children"`
}

// exploreOut is an internal structure for unmarshaling the Explore API response.
type exploreOut struct {
	Widgets []*ExploreWidget `json:"widgets" bson:"widgets"`
}

// ExploreWidget represents a single widget returned by the Explore function.
// Each widget corresponds to a specific type of data visualization and contains
// the token and request information needed to fetch detailed data.
//
// Widget types include:
//   - TIMESERIES: Interest over time data (use with InterestOverTime)
//   - GEO_MAP: Geographic interest data (use with InterestByLocation)
//   - RELATED_QUERIES: Related search queries (use with Related)
//   - RELATED_TOPICS: Related topics (use with Related)
type ExploreWidget struct {
	// Token is the authentication token required to fetch this widget's data.
	Token string `json:"token" bson:"token"`

	// Type describes the widget type (e.g., "fe_line_chart", "fe_geo_chart").
	Type string `json:"type" bson:"type"`

	// Title is the display title for this widget (e.g., "Interest over time").
	Title string `json:"title" bson:"title"`

	// ID is the widget identifier that indicates its type and order.
	// Format: "{WIDGET_TYPE}" or "{WIDGET_TYPE}_{INDEX}" for comparison items.
	ID string `json:"id" bson:"id"`

	// Request contains the parameters needed to fetch this widget's data.
	Request *WidgetResponse `json:"request" bson:"request"`
}

// ExploreResponse is a slice of ExploreWidget pointers returned by the Explore function.
// It implements sort.Interface for sorting widgets by their order index.
type ExploreResponse []*ExploreWidget

// Sort sorts the widgets in ascending order by their index suffix.
// Widgets without an index suffix are placed first.
func (e ExploreResponse) Sort() {
	sort.Sort(e)
}

// Len returns the number of widgets in the response.
// This method is part of the sort.Interface implementation.
func (e ExploreResponse) Len() int {
	return len(e)
}

// Less reports whether widget at index i should sort before widget at index j.
// Sorting is based on the numeric suffix in widget IDs (e.g., "RELATED_QUERIES_0" < "RELATED_QUERIES_1").
// This method is part of the sort.Interface implementation.
func (e ExploreResponse) Less(i, j int) bool {
	numI := strings.LastIndex(e[i].ID, "_")
	if numI < 0 {
		return true
	}

	numJ := strings.LastIndex(e[j].ID, "_")
	if numJ < 0 {
		return false
	}

	valI, err := strconv.ParseInt(e[i].ID[numI+1:], 10, 32)
	if err != nil {
		return true
	}

	valJ, err := strconv.ParseInt(e[j].ID[numJ+1:], 10, 32)
	if err != nil {
		return false
	}

	return valI < valJ
}

// Swap exchanges the widgets at indices i and j.
// This method is part of the sort.Interface implementation.
func (e ExploreResponse) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

// GetWidgetsByOrder returns widgets that match the specified comparison item index.
// Use this to get related queries/topics for a specific keyword in a multi-keyword comparison.
//
// Example:
//
//	// For a 3-keyword comparison, get related queries for the second keyword:
//	widgets := response.GetWidgetsByOrder(1)
//
// Note: Widgets without an index (TIMESERIES, GEO_MAP) are excluded from results.
func (e ExploreResponse) GetWidgetsByOrder(i int) ExploreResponse {
	out := make(ExploreResponse, 0)
	for _, v := range e {
		if v.ID == string(IntOverTimeWidgetID) || v.ID == string(IntOverRegionID) {
			continue
		}

		ind := strings.LastIndex(v.ID, "_")
		val, err := strconv.ParseInt(v.ID[ind+1:], 10, 32)
		if err != nil {
			return out
		}

		if int(val) == i {
			out = append(out, v)
		}
	}

	return out
}

// GetWidgetsByType returns all widgets matching the specified WidgetType.
// Use this to filter widgets for specific data retrieval functions.
//
// Example:
//
//	// Get all interest over time widgets:
//	timeWidgets := response.GetWidgetsByType(IntOverTimeWidgetID)
//
//	// Get all related queries widgets:
//	queryWidgets := response.GetWidgetsByType(RelatedQueriesID)
func (e ExploreResponse) GetWidgetsByType(t WidgetType) ExploreResponse {
	out := make(ExploreResponse, 0)
	for _, v := range e {
		if strings.Contains(v.ID, string(t)) {
			out = append(out, v)
		}
	}

	return out
}

// WidgetResponse contains the request parameters for fetching widget data.
// This structure is embedded in ExploreWidget and contains system-level
// configuration for each type of trends search.
type WidgetResponse struct {
	// Geo is the geographic filter configuration.
	// Can be a map or other structure depending on the request type.
	Geo interface{} `json:"geo,omitempty" bson:"geo"`

	// Time is the time range specification for the data.
	Time string `json:"time,omitempty" bson:"time"`

	// Resolution specifies the data granularity (e.g., "DAY", "WEEK", "MONTH").
	Resolution string `json:"resolution,omitempty" bson:"resolution"`

	// Locale is the locale code for localized results.
	Locale string `json:"locale,omitempty" bson:"locale"`

	// Restriction contains filtering restrictions for the request.
	Restriction WidgetComparisonItem `json:"restriction" bson:"restriction"`

	// CompItem contains comparison items for multi-keyword comparisons.
	CompItem []*WidgetComparisonItem `json:"comparisonItem" bson:"comparison_item"`

	// RequestOpt contains additional request options.
	RequestOpt RequestOptions `json:"requestOptions" bson:"request_option"`

	// KeywordType specifies the type of keyword (e.g., "QUERY", "ENTITY").
	KeywordType string `json:"keywordType" bson:"keyword_type"`

	// Metric specifies which metrics to include in the response.
	Metric []string `json:"metric" bson:"metric"`

	// Language is the language code for results.
	Language string `json:"language" bson:"language"`

	// TrendinessSettings contains trendiness calculation settings.
	TrendinessSettings map[string]string `json:"trendinessSettings" bson:"trendiness_settings"`

	// DataMode specifies the data mode (e.g., "PERCENTAGES" for comparisons).
	DataMode string `json:"dataMode,omitempty" bson:"data_mode"`

	// UserConfig contains user-specific configuration settings.
	UserConfig map[string]string `json:"userConfig,omitempty" bson:"user_config"`

	// UserCountryCode is the user's country code for localization.
	UserCountryCode string `json:"userCountryCode,omitempty" bson:"user_country_code"`
}

// WidgetComparisonItem contains comparison-specific parameters for widget requests.
// This structure is used both as a restriction filter and as individual comparison items.
type WidgetComparisonItem struct {
	// Geo is a map of geographic filters (key-value pairs for region codes).
	Geo map[string]string `json:"geo,omitempty" bson:"geo"`

	// Time is the time range for this comparison item.
	Time string `json:"time,omitempty" bson:"time"`

	// ComplexKeywordsRestriction contains keyword-specific restrictions.
	ComplexKeywordsRestriction KeywordsRestriction `json:"complexKeywordsRestriction,omitempty" bson:"complex_keywords_restriction"`

	// OriginalTimeRangeForExploreURL preserves the original time range for URL generation.
	OriginalTimeRangeForExploreURL string `json:"originalTimeRangeForExploreUrl,omitempty" bson:"original_time_range_for_explore_url"`
}

// KeywordsRestriction contains a list of keyword restrictions for filtering results.
// This is used internally by Google Trends for advanced filtering.
type KeywordsRestriction struct {
	// Keyword is the list of individual keyword restrictions.
	Keyword []*KeywordRestriction `json:"keyword" bson:"keyword"`
}

// KeywordRestriction defines a single keyword filter restriction.
// It specifies both the type of restriction and the value to filter by.
type KeywordRestriction struct {
	// Type is the restriction type (e.g., "BROAD", "PHRASE", "EXACT").
	Type string `json:"type" bson:"type"`

	// Value is the keyword value for this restriction.
	Value string `json:"value" bson:"value"`
}

// RequestOptions contains additional options for widget data requests.
// These options affect how results are filtered and processed.
type RequestOptions struct {
	// Property specifies the Google property (e.g., "", "youtube", "news").
	Property string `json:"property" bson:"property"`

	// Backend specifies the backend service to use for the request.
	Backend string `json:"backend" bson:"backend"`

	// Category is the category ID filter for results.
	Category int `json:"category" bson:"category"`
}

// multilineOut is an internal structure for unmarshaling interest over time API responses.
type multilineOut struct {
	Default multiline `json:"default" bson:"default"`
}

// multiline is an internal structure containing timeline data.
type multiline struct {
	TimelineData []*Timeline `json:"timelineData" bson:"timeline_data"`
}

// Timeline represents a single data point in the interest over time chart.
// It contains timestamp, values for each compared keyword, and formatted display strings.
//
// For multi-keyword comparisons, the Value, HasData, and FormattedValue slices
// contain one element per keyword in the same order as the comparison items.
type Timeline struct {
	// Time is the Unix timestamp (in seconds) for this data point.
	Time string `json:"time" bson:"time"`

	// FormattedTime is a human-readable timestamp (e.g., "Jan 1, 2020").
	FormattedTime string `json:"formattedTime" bson:"formatted_time"`

	// FormattedAxisTime is a shortened time string for chart axis labels.
	FormattedAxisTime string `json:"formattedAxisTime" bson:"formatted_axis_time"`

	// Value contains interest values (0-100) for each keyword at this time point.
	// Values are relative, with 100 representing peak popularity.
	Value []int `json:"value" bson:"value"`

	// HasData indicates whether data is available for each keyword at this time.
	HasData []bool `json:"hasData" bson:"has_data"`

	// FormattedValue contains display-ready strings for each value.
	FormattedValue []string `json:"formattedValue" bson:"formatted_value"`
}

// geoOut is an internal structure for unmarshaling interest by location API responses.
type geoOut struct {
	Default geo `json:"default" bson:"default"`
}

// geo is an internal structure containing geographic map data.
type geo struct {
	GeoMapData []*GeoMap `json:"geoMapData" bson:"geomap_data"`
}

// GeoMap represents interest data for a single geographic location.
// It is used to create choropleth maps showing regional interest distribution.
//
// For multi-keyword comparisons, the Value, FormattedValue, and HasData slices
// contain one element per keyword in the same order as the comparison items.
type GeoMap struct {
	// GeoCode is the geographic code for this region (e.g., "US-CA", "GB-ENG").
	GeoCode string `json:"geoCode" bson:"geo_code"`

	// GeoName is the human-readable name for this region (e.g., "California", "England").
	GeoName string `json:"geoName" bson:"geo_name"`

	// Value contains interest values (0-100) for each keyword in this region.
	// Values are relative, with 100 representing the region with highest interest.
	Value []int `json:"value" bson:"value"`

	// FormattedValue contains display-ready strings for each value.
	FormattedValue []string `json:"formattedValue" bson:"formatted_value"`

	// MaxValueIndex indicates which keyword has the highest value in this region.
	// Useful for coloring maps in multi-keyword comparisons.
	MaxValueIndex int `json:"maxValueIndex" bson:"max_value_index"`

	// HasData indicates whether data is available for each keyword in this region.
	HasData []bool `json:"hasData" bson:"has_data"`
}

// relatedOut is an internal structure for unmarshaling related searches API responses.
type relatedOut struct {
	Default relatedList `json:"default" bson:"default"`
}

// relatedList is an internal structure containing ranked keyword lists.
type relatedList struct {
	Ranked []*rankedList `json:"rankedList" bson:"ranked"`
}

// rankedList is an internal structure containing a list of ranked keywords.
type rankedList struct {
	Keywords []*RankedKeyword `json:"rankedKeyword" bson:"keywords"`
}

// searchOut is an internal structure for unmarshaling autocomplete API responses.
type searchOut struct {
	Default searchList `json:"default" bson:"default"`
}

// searchList is an internal structure containing keyword topic suggestions.
type searchList struct {
	Keywords []*KeywordTopic `json:"topics" bson:"keywords"`
}

// RankedKeyword represents a related search query or topic with its ranking value.
// This is returned by the Related function for both related queries and related topics.
//
// For related queries, the Query field contains the search term.
// For related topics, the Topic field contains the topic details.
type RankedKeyword struct {
	// Query is the related search query string (for related queries only).
	Query string `json:"query,omitempty" bson:"query"`

	// Topic contains details about the related topic (for related topics only).
	Topic KeywordTopic `json:"topic,omitempty" bson:"topic"`

	// Value is the relative interest value (0-100).
	// Can also be displayed as "+X%" for rising queries.
	Value int `json:"value" bson:"value"`

	// FormattedValue is a display-ready string (e.g., "100", "+250%", "Breakout").
	FormattedValue string `json:"formattedValue" bson:"formatted_value"`

	// HasData indicates whether interest data is available for this keyword.
	HasData bool `json:"hasData" bson:"has_data"`

	// Link is the Google Trends URL for exploring this related query/topic.
	Link string `json:"link" bson:"link"`
}

// KeywordTopic represents a Google Knowledge Graph topic or entity.
// Topics are used in autocomplete suggestions and related topics results.
//
// Example:
//
//	KeywordTopic{Mid: "/m/09c7w0", Title: "Go", Type: "Programming language"}
type KeywordTopic struct {
	// Mid is the Google Knowledge Graph machine ID for this topic.
	// This can be used for more precise searches instead of keywords.
	Mid string `json:"mid" bson:"mid"`

	// Title is the human-readable name of the topic.
	Title string `json:"title" bson:"title"`

	// Type describes the category of this topic (e.g., "Programming language", "City").
	Type string `json:"type" bson:"type"`
}
