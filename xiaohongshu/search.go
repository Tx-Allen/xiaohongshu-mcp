package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
)

type SearchResult struct {
	Search struct {
		Feeds FeedsValue `json:"feeds"`
	} `json:"search"`
}

type SearchAction struct {
	page *rod.Page
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string) ([]Feed, error) {
	page := s.page.Context(ctx)

	searchURL := makeSearchURL(keyword)
	if err := page.Navigate(searchURL); err != nil {
		return nil, err
	}

	if err := waitForInitialState(page, `() => {
		const state = window.__INITIAL_STATE__;
		return !!(
			state &&
			state.search &&
			state.search.feeds &&
			state.search.feeds._value &&
			state.search.feeds._value.length > 0
		);
	}`, 30*time.Second); err != nil {
		return nil, err
	}

	// 获取 window.__INITIAL_STATE__ 并转换为 JSON 字符串
	result, err := page.Evaluate(&rod.EvalOptions{JS: `() => {
		if (window.__INITIAL_STATE__) {
			return JSON.stringify(window.__INITIAL_STATE__);
		}
		return "";
	}`, ByValue: true})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("failed to evaluate search initial state")
	}

	str := result.Value.Str()

	if str == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	var searchResult SearchResult
	if err := json.Unmarshal([]byte(str), &searchResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	return searchResult.Search.Feeds.Value, nil
}

func makeSearchURL(keyword string) string {

	values := url.Values{}
	values.Set("keyword", keyword)
	values.Set("source", "web_explore_feed")

	return fmt.Sprintf("https://www.xiaohongshu.com/search_result?%s", values.Encode())
}
