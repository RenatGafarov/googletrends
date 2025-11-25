package googletrends

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var client = newGClient()

// Debug allows to see request-response details.
func Debug(debug bool) {
	client.debug = debug
}


// Daily gets daily trends descending ordered by days and articles corresponding to it.
// This is an alias for DailyNew which uses the new Google Trends API.
func Daily(ctx context.Context, hl, loc string) ([]*TrendingSearch, error) {
	return DailyNew(ctx, hl, loc)
}



// ExploreCategories gets available categories for explore and comparison and caches it in client.
// Deprecated: This function uses the old Google Trends API which may be unstable.
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

// ExploreLocations gets available locations for explore and comparison and caches it in client.
// Deprecated: This function uses the old Google Trends API which may be unstable.
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

// Explore list of widgets with tokens. Every widget
// is related to specific method (`InterestOverTime`, `InterestOverLoc`, `RelatedSearches`, `Suggestions`)
// and contains required token and request information.
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

// InterestOverTime as list of `Timeline` dots for chart.
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

// InterestByLocation as list of `GeoMap`, with geo codes and interest values.
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

// Related topics or queries, list of `RankedKeyword`, supports two types of widgets.
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

// Related topics or queries, list of `RankedKeyword`, supports two types of widgets.
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

// DailyNew gets daily trends using the new Google Trends API.
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

// DailyTrendingSearchNew gets daily trends ordered by days using the new Google Trends API.
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
