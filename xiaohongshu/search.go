package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

// SearchFilters 定义搜索可选筛选项
type SearchFilters struct {
	Sort        string
	NoteType    string
	PublishTime string
	SearchScope string
	Distance    string
}

const (
	SortDefault       = "comprehensive"
	SortLatest        = "latest"
	SortMostLikes     = "most_likes"
	SortMostComments  = "most_comments"
	SortMostFavorites = "most_favorites"

	NoteTypeAll   = "all"
	NoteTypeVideo = "video"
	NoteTypeImage = "image"

	PublishAll    = "all"
	PublishDay    = "day"
	PublishWeek   = "week"
	PublishHalfYr = "half_year"

	ScopeAll      = "all"
	ScopeSeen     = "seen"
	ScopeUnseen   = "unseen"
	ScopeFollowed = "followed"

	DistanceAll      = "all"
	DistanceSameCity = "same_city"
	DistanceNearby   = "nearby"
)

var sortOptionLabels = map[string]string{
	SortDefault:       "综合",
	SortLatest:        "最新",
	SortMostLikes:     "最多点赞",
	SortMostComments:  "最多评论",
	SortMostFavorites: "最多收藏",
}

var noteTypeLabels = map[string]string{
	NoteTypeAll:   "不限",
	NoteTypeVideo: "视频",
	NoteTypeImage: "图文",
}

var publishTimeLabels = map[string]string{
	PublishAll:    "不限",
	PublishDay:    "一天内",
	PublishWeek:   "一周内",
	PublishHalfYr: "半年内",
}

var searchScopeLabels = map[string]string{
	ScopeAll:      "不限",
	ScopeSeen:     "已看过",
	ScopeUnseen:   "未看过",
	ScopeFollowed: "已关注",
}

var distanceLabels = map[string]string{
	DistanceAll:      "不限",
	DistanceSameCity: "同城",
	DistanceNearby:   "附近",
}

// NewSearchFilters 构建筛选器，若值为空则回退到默认
func NewSearchFilters(sort, noteType, publishTime, searchScope, distance string) (*SearchFilters, error) {
	if sort == "" {
		sort = SortDefault
	}
	if noteType == "" {
		noteType = NoteTypeAll
	}
	if publishTime == "" {
		publishTime = PublishAll
	}
	if searchScope == "" {
		searchScope = ScopeAll
	}
	if distance == "" {
		distance = DistanceAll
	}

	if _, ok := sortOptionLabels[sort]; !ok {
		return nil, fmt.Errorf("invalid sort option: %s", sort)
	}
	if _, ok := noteTypeLabels[noteType]; !ok {
		return nil, fmt.Errorf("invalid note_type option: %s", noteType)
	}
	if _, ok := publishTimeLabels[publishTime]; !ok {
		return nil, fmt.Errorf("invalid publish_time option: %s", publishTime)
	}
	if _, ok := searchScopeLabels[searchScope]; !ok {
		return nil, fmt.Errorf("invalid search_scope option: %s", searchScope)
	}
	if _, ok := distanceLabels[distance]; !ok {
		return nil, fmt.Errorf("invalid distance option: %s", distance)
	}

	return &SearchFilters{
		Sort:        sort,
		NoteType:    noteType,
		PublishTime: publishTime,
		SearchScope: searchScope,
		Distance:    distance,
	}, nil
}

func (f *SearchFilters) isDefault() bool {
	if f == nil {
		return true
	}
	return f.Sort == SortDefault &&
		f.NoteType == NoteTypeAll &&
		f.PublishTime == PublishAll &&
		f.SearchScope == ScopeAll &&
		f.Distance == DistanceAll
}

func NewSearchAction(page *rod.Page) *SearchAction {
	pp := page.Timeout(60 * time.Second)

	return &SearchAction{page: pp}
}

func (s *SearchAction) Search(ctx context.Context, keyword string, filters *SearchFilters) ([]Feed, error) {
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

	if filters != nil && !filters.isDefault() {
		if err := applySearchFilters(page, filters); err != nil {
			return nil, err
		}
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

func applySearchFilters(page *rod.Page, filters *SearchFilters) error {
	filterBtn := page.MustElement(`div.filter`)
	filterBtn.MustHover()
	panel := page.MustElement(`div.filter-panel`).MustWaitVisible()

	if filters.Sort != SortDefault {
		if err := clickFilterTag(panel, `.filters-wrapper > div:nth-child(1) .tags`, sortOptionLabels[filters.Sort]); err != nil {
			return err
		}
	}

	if filters.NoteType != NoteTypeAll {
		if err := clickFilterTag(panel, `.filters-wrapper > div:nth-child(2) .tags`, noteTypeLabels[filters.NoteType]); err != nil {
			return err
		}
	}

	if filters.PublishTime != PublishAll {
		if err := clickFilterTag(panel, `.filters-wrapper > div:nth-child(3) .tags`, publishTimeLabels[filters.PublishTime]); err != nil {
			return err
		}
	}

	if filters.SearchScope != ScopeAll {
		if err := clickFilterTag(panel, `.filters-wrapper > div:nth-child(4) .tags`, searchScopeLabels[filters.SearchScope]); err != nil {
			return err
		}
	}

	if filters.Distance != DistanceAll {
		if err := clickFilterTag(panel, `.filters-wrapper > div:nth-child(5) .tags`, distanceLabels[filters.Distance]); err != nil {
			return err
		}
	}

	panel.MustElement(`.operation-container .operation:nth-child(2)`).MustClick()
	time.Sleep(500 * time.Millisecond)
	return waitForInitialState(page, `() => {
		const state = window.__INITIAL_STATE__;
		return !!(
			state &&
			state.search &&
			state.search.feeds &&
			state.search.feeds._value &&
			state.search.feeds._value.length > 0
		);
	}`, 30*time.Second)
}

func clickFilterTag(panel *rod.Element, selector, target string) error {
	tags := panel.MustElements(selector)
	for _, tag := range tags {
		textEl, err := tag.Element("span")
		if err != nil || textEl == nil {
			continue
		}
		text := strings.TrimSpace(textEl.MustText())
		if text == target {
			className, _ := tag.Attribute("class")
			if className != nil && strings.Contains(*className, "active") {
				return nil
			}
			tag.MustClick()
			time.Sleep(200 * time.Millisecond)
			return nil
		}
	}
	return fmt.Errorf("未找到筛选项 %s", target)
}
