package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

type FeedsListAction struct {
	page *rod.Page
}

// FeedsResult 定义页面初始状态结构
type FeedsResult struct {
	Feed FeedData `json:"feed"`
}

func NewFeedsListAction(page *rod.Page) (*FeedsListAction, error) {
	pp := page.Timeout(60 * time.Second)

	if err := pp.Navigate("https://www.xiaohongshu.com"); err != nil {
		return nil, err
	}

	if err := waitForInitialState(pp, `() => {
		const state = window.__INITIAL_STATE__;
		return !!(state && state.feed && state.feed.feeds && state.feed.feeds._value);
	}`, 30*time.Second); err != nil {
		return nil, err
	}

	return &FeedsListAction{page: pp}, nil
}

// GetFeedsList 获取页面的 Feed 列表数据
func (f *FeedsListAction) GetFeedsList(ctx context.Context) ([]Feed, error) {
	page := f.page.Context(ctx)

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
		return nil, fmt.Errorf("failed to evaluate feeds initial state")
	}

	jsonStr := result.Value.Str()

	if jsonStr == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}

	// 解析完整的 InitialState
	var state FeedsResult
	if err := json.Unmarshal([]byte(jsonStr), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}

	// 返回 feed.feeds._value
	return state.Feed.Feeds.Value, nil
}
