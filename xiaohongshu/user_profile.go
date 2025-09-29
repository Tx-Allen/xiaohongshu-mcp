package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

type UserProfileAction struct {
	page *rod.Page
}

func NewUserProfileAction(page *rod.Page) *UserProfileAction {
	pp := page.Timeout(60 * time.Second)
	return &UserProfileAction{page: pp}
}

// UserProfile 获取用户基本信息及帖子
func (u *UserProfileAction) UserProfile(ctx context.Context, userID, xsecToken string) (*UserProfileResponse, error) {
	page := u.page.Context(ctx)

	searchURL := makeUserProfileURL(userID, xsecToken)
	if err := page.Navigate(searchURL); err != nil {
		return nil, err
	}

	if err := waitForInitialState(page, `() => {
		const state = window.__INITIAL_STATE__;
		return !!(state && state.user && state.user.userPageData);
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
		return nil, fmt.Errorf("failed to evaluate user profile initial state")
	}

	jsonStr := result.Value.Str()

	if jsonStr == "" {
		return nil, fmt.Errorf("__INITIAL_STATE__ not found")
	}
	// 定义响应结构并直接反序列化
	var initialState = struct {
		User struct {
			UserPageData UserPageData `json:"userPageData"`
			Notes        struct {
				Feeds [][]Feed `json:"_rawValue"` // 帖子为双重数组
			} `json:"notes"`
		} `json:"user"`
	}{}
	if err := json.Unmarshal([]byte(jsonStr), &initialState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal __INITIAL_STATE__: %w", err)
	}
	response := &UserProfileResponse{
		UserBasicInfo: initialState.User.UserPageData.RawValue.BasicInfo,
		Interactions:  initialState.User.UserPageData.RawValue.Interactions,
	}
	// 添加用户贴子
	for _, feeds := range initialState.User.Notes.Feeds {
		if len(feeds) != 0 {
			response.Feeds = append(response.Feeds, feeds...)
		}
	}

	return response, nil

}

func makeUserProfileURL(userID, xsecToken string) string {
	return fmt.Sprintf("https://www.xiaohongshu.com/user/profile/%s?xsec_token=%s&xsec_source=pc_note", userID, xsecToken)
}
