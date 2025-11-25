package googletrends

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrendsCategories(t *testing.T) {
	t.Parallel()

	categories := TrendsCategories()

	assert.NotNil(t, categories)
	assert.Greater(t, len(categories), 0)

	// Verify expected categories exist
	expectedCategories := []string{"all", "b", "h", "m", "t", "e", "s"}
	for _, cat := range expectedCategories {
		_, exists := categories[cat]
		assert.True(t, exists, "Expected category %s to exist", cat)
	}
}

func TestExploreResponseSort(t *testing.T) {
	t.Parallel()

	widgets := ExploreResponse{
		{ID: "RELATED_QUERIES_2"},
		{ID: "RELATED_TOPICS_0"},
		{ID: "RELATED_QUERIES_1"},
		{ID: "TIMESERIES"},
		{ID: "GEO_MAP"},
	}

	widgets.Sort()

	// Verify the order after sorting
	assert.Equal(t, "TIMESERIES", widgets[0].ID)
	assert.Equal(t, "GEO_MAP", widgets[1].ID)
}

func TestExploreResponseLen(t *testing.T) {
	t.Parallel()

	widgets := ExploreResponse{
		{ID: "widget1"},
		{ID: "widget2"},
		{ID: "widget3"},
	}

	assert.Equal(t, 3, widgets.Len())
}

func TestExploreResponseSwap(t *testing.T) {
	t.Parallel()

	widgets := ExploreResponse{
		{ID: "widget1"},
		{ID: "widget2"},
	}

	widgets.Swap(0, 1)

	assert.Equal(t, "widget2", widgets[0].ID)
	assert.Equal(t, "widget1", widgets[1].ID)
}

func TestExploreResponseGetWidgetsByOrder(t *testing.T) {
	t.Parallel()

	widgets := ExploreResponse{
		{ID: "RELATED_QUERIES_0"},
		{ID: "RELATED_TOPICS_0"},
		{ID: "RELATED_QUERIES_1"},
		{ID: "RELATED_TOPICS_1"},
		{ID: "TIMESERIES"},
		{ID: "GEO_MAP"},
	}

	// Get widgets for order 0
	order0 := widgets.GetWidgetsByOrder(0)
	assert.Equal(t, 2, len(order0))

	// Get widgets for order 1
	order1 := widgets.GetWidgetsByOrder(1)
	assert.Equal(t, 2, len(order1))

	// Get widgets for non-existent order
	order99 := widgets.GetWidgetsByOrder(99)
	assert.Equal(t, 0, len(order99))
}

func TestExploreResponseGetWidgetsByType(t *testing.T) {
	t.Parallel()

	widgets := ExploreResponse{
		{ID: "RELATED_QUERIES_0"},
		{ID: "RELATED_TOPICS_0"},
		{ID: "RELATED_QUERIES_1"},
		{ID: "TIMESERIES"},
		{ID: "GEO_MAP"},
	}

	// Get RELATED_QUERIES widgets
	queries := widgets.GetWidgetsByType(RelatedQueriesID)
	assert.Equal(t, 2, len(queries))

	// Get RELATED_TOPICS widgets
	topics := widgets.GetWidgetsByType(RelatedTopicsID)
	assert.Equal(t, 1, len(topics))

	// Get TIMESERIES widgets
	timeseries := widgets.GetWidgetsByType(IntOverTimeWidgetID)
	assert.Equal(t, 1, len(timeseries))
}

func TestWidgetTypes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, WidgetType("TIMESERIES"), IntOverTimeWidgetID)
	assert.Equal(t, WidgetType("GEO_MAP"), IntOverRegionID)
	assert.Equal(t, WidgetType("RELATED_QUERIES"), RelatedQueriesID)
	assert.Equal(t, WidgetType("RELATED_TOPICS"), RelatedTopicsID)
}

func TestErrorVariables(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, ErrInvalidCategory)
	assert.NotNil(t, ErrRequestFailed)
	assert.NotNil(t, ErrInvalidWidgetType)

	assert.Equal(t, "invalid category param", ErrInvalidCategory.Error())
	assert.Equal(t, "failed to perform http request", ErrRequestFailed.Error())
	assert.Equal(t, "invalid widget type", ErrInvalidWidgetType.Error())
}

func TestTrendingSearchStructs(t *testing.T) {
	t.Parallel()

	search := &TrendingSearch{
		Title: &SearchTitle{
			Query: "test query",
		},
		FormattedTraffic: "100K+",
		Image: &SearchImage{
			NewsURL:  "https://news.example.com",
			Source:   "Example News",
			ImageURL: "https://img.example.com/image.jpg",
		},
		Articles: []*SearchArticle{
			{
				Title:   "Test Article",
				TimeAgo: "1h ago",
				Source:  "Example Source",
				URL:     "https://example.com/article",
				Snippet: "Test snippet",
			},
		},
	}

	assert.Equal(t, "test query", search.Title.Query)
	assert.Equal(t, "100K+", search.FormattedTraffic)
	assert.Equal(t, "Example News", search.Image.Source)
	assert.Equal(t, 1, len(search.Articles))
	assert.Equal(t, "Test Article", search.Articles[0].Title)
}

func TestExploreRequest(t *testing.T) {
	t.Parallel()

	req := &ExploreRequest{
		ComparisonItems: []*ComparisonItem{
			{
				Keyword: "golang",
				Geo:     "US",
				Time:    "today 12-m",
			},
		},
		Category: 31,
		Property: "",
	}

	assert.Equal(t, 1, len(req.ComparisonItems))
	assert.Equal(t, "golang", req.ComparisonItems[0].Keyword)
	assert.Equal(t, "US", req.ComparisonItems[0].Geo)
	assert.Equal(t, 31, req.Category)
}

func TestExploreCatTree(t *testing.T) {
	t.Parallel()

	tree := &ExploreCatTree{
		Name: "Programming",
		ID:   31,
		Children: []*ExploreCatTree{
			{Name: "Go", ID: 100},
			{Name: "Python", ID: 101},
		},
	}

	assert.Equal(t, "Programming", tree.Name)
	assert.Equal(t, 31, tree.ID)
	assert.Equal(t, 2, len(tree.Children))
	assert.Equal(t, "Go", tree.Children[0].Name)
}

func TestExploreLocTree(t *testing.T) {
	t.Parallel()

	tree := &ExploreLocTree{
		Name: "United States",
		ID:   "US",
		Children: []*ExploreLocTree{
			{Name: "California", ID: "US-CA"},
			{Name: "New York", ID: "US-NY"},
		},
	}

	assert.Equal(t, "United States", tree.Name)
	assert.Equal(t, "US", tree.ID)
	assert.Equal(t, 2, len(tree.Children))
	assert.Equal(t, "California", tree.Children[0].Name)
}

func TestTimeline(t *testing.T) {
	t.Parallel()

	timeline := &Timeline{
		Time:              "1234567890",
		FormattedTime:     "Jan 1, 2021",
		FormattedAxisTime: "Jan 2021",
		Value:             []int{50, 75, 100},
		HasData:           []bool{true, true, true},
		FormattedValue:    []string{"50", "75", "100"},
	}

	assert.Equal(t, "1234567890", timeline.Time)
	assert.Equal(t, 3, len(timeline.Value))
	assert.Equal(t, 100, timeline.Value[2])
}

func TestGeoMap(t *testing.T) {
	t.Parallel()

	geoMap := &GeoMap{
		GeoCode:        "US-CA",
		GeoName:        "California",
		Value:          []int{100},
		FormattedValue: []string{"100"},
		MaxValueIndex:  0,
		HasData:        []bool{true},
	}

	assert.Equal(t, "US-CA", geoMap.GeoCode)
	assert.Equal(t, "California", geoMap.GeoName)
	assert.Equal(t, 100, geoMap.Value[0])
}

func TestRankedKeyword(t *testing.T) {
	t.Parallel()

	keyword := &RankedKeyword{
		Query:          "golang tutorial",
		Value:          100,
		FormattedValue: "100",
		HasData:        true,
		Link:           "/trends/explore?q=golang+tutorial",
		Topic: KeywordTopic{
			Mid:   "/m/09gbxjr",
			Title: "Go",
			Type:  "Programming language",
		},
	}

	assert.Equal(t, "golang tutorial", keyword.Query)
	assert.Equal(t, 100, keyword.Value)
	assert.Equal(t, "/m/09gbxjr", keyword.Topic.Mid)
}

func TestKeywordTopic(t *testing.T) {
	t.Parallel()

	topic := &KeywordTopic{
		Mid:   "/m/09gbxjr",
		Title: "Go",
		Type:  "Programming language",
	}

	assert.Equal(t, "/m/09gbxjr", topic.Mid)
	assert.Equal(t, "Go", topic.Title)
	assert.Equal(t, "Programming language", topic.Type)
}
