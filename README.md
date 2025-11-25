# Google Trends API for Go

Unofficial Google Trends API for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/RenatGafarov/googletrends.svg)](https://pkg.go.dev/github.com/RenatGafarov/googletrends)
[![Go Report Card](https://goreportcard.com/badge/github.com/RenatGafarov/googletrends)](https://goreportcard.com/report/github.com/RenatGafarov/googletrends)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Installation

```bash
go get github.com/RenatGafarov/googletrends
```

**Requirements:** Go 1.23+

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/RenatGafarov/googletrends"
)

func main() {
    ctx := context.Background()

    // Get daily trending searches
    trends, err := googletrends.DailyNew(ctx, "EN", "US")
    if err != nil {
        log.Fatal(err)
    }

    for _, t := range trends {
        log.Println(t.Title.Query)
    }
}
```

## Debug Mode

Enable debug mode to see request/response details:

```go
googletrends.Debug(true)
```

## API Methods

### Daily Trends (Recommended)

```go
// Get daily trending searches
trends, err := googletrends.DailyNew(ctx, "EN", "US")

// Get daily trends grouped by days
trendsByDays, err := googletrends.DailyTrendingSearchNew(ctx, "EN", "US")
```

### Explore & Analytics

```go
// Search for keyword suggestions
keywords, err := googletrends.Search(ctx, "Go", "EN")

// Get explore widgets for a keyword
explore, err := googletrends.Explore(ctx, &googletrends.ExploreRequest{
    ComparisonItems: []*googletrends.ComparisonItem{
        {
            Keyword: "Go",
            Geo:     "US",
            Time:    "today 12-m",
        },
    },
    Category: 31, // Programming category
    Property: "",
}, "EN")

// Interest over time (for charts)
timeline, err := googletrends.InterestOverTime(ctx, explore[0], "EN")

// Interest by location (for maps)
geoData, err := googletrends.InterestByLocation(ctx, explore[1], "EN")

// Related topics
topics, err := googletrends.Related(ctx, explore[2], "EN")

// Related queries
queries, err := googletrends.Related(ctx, explore[3], "EN")
```

### Compare Keywords

```go
compare, err := googletrends.Explore(ctx, &googletrends.ExploreRequest{
    ComparisonItems: []*googletrends.ComparisonItem{
        {Keyword: "Go", Geo: "US", Time: "today 12-m"},
        {Keyword: "Python", Geo: "US", Time: "today 12-m"},
        {Keyword: "Rust", Geo: "US", Time: "today 12-m"},
    },
    Category: 31,
    Property: "",
}, "EN")
```

### Legacy Methods (Deprecated)

The following methods use the old Google Trends API and may be unstable:

- `Daily()` - use `DailyNew()` instead
- `DailyTrendingSearch()` - use `DailyTrendingSearchNew()` instead
- `Realtime()` - realtime trends (limited availability)
- `ExploreCategories()` - get category tree
- `ExploreLocations()` - get location tree
- `TrendsCategories()` - available categories for realtime trends

## Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `hl` | User interface language | `"EN"`, `"RU"`, `"DE"` |
| `loc` | Location (geo) country code | `"US"`, `"GB"`, `"DE"` |
| `cat` | Category for realtime trends | `"all"`, `"b"` (business), `"t"` (tech) |

### Time Ranges

Common time range formats for `ComparisonItem.Time`:

- `"now 1-H"` - last hour
- `"now 4-H"` - last 4 hours
- `"now 1-d"` - last day
- `"now 7-d"` - last 7 days
- `"today 1-m"` - last month
- `"today 3-m"` - last 3 months
- `"today 12-m"` - last 12 months
- `"today 5-y"` - last 5 years
- `"all"` - all time (2004-present)

## Examples

See the [example](./example) directory for complete working examples.

## License

[MIT License](LICENSE)
