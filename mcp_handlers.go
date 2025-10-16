package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/accounts"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// MCP 工具处理函数

func accountIDFromArgs(args map[string]interface{}) (string, error) {
	if args == nil {
		return "", accounts.ErrMissingAccountID
	}

	raw, ok := args["account_id"].(string)
	if !ok {
		return "", accounts.ErrMissingAccountID
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", accounts.ErrMissingAccountID
	}

	return accounts.ResolveAccountID(trimmed)
}

func accountErrorResult(err error) *MCPToolResult {
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: fmt.Sprintf("账号参数错误: %v", err),
		}},
		IsError: true,
	}
}

func stringFromArgs(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stringSliceFromArgs(args map[string]interface{}, key string) []string {
	result := make([]string, 0)
	if args == nil {
		return result
	}
	value, ok := args[key]
	if !ok {
		return result
	}

	switch items := value.(type) {
	case []interface{}:
		for _, item := range items {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					result = append(result, s)
				}
			}
		}
	case []string:
		for _, s := range items {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
	}

	return result
}

// handleCheckLoginStatus 处理检查登录状态
func (s *AppServer) handleCheckLoginStatus(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 检查登录状态")

	status, err := s.xiaohongshuService.CheckLoginStatus(ctx, accountID)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "检查登录状态失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	resultText := fmt.Sprintf("账号 %s 登录状态检查成功: %+v", accountID, status)
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: resultText,
		}},
	}
}

// handleGetLoginQrcode 处理获取登录二维码请求。
// 返回二维码图片的 Base64 编码和超时时间，供前端展示扫码登录。
func (s *AppServer) handleGetLoginQrcode(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 获取登录扫码图片")

	result, err := s.xiaohongshuService.GetLoginQrcode(ctx, accountID)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "获取登录扫码图片失败: " + err.Error()}},
			IsError: true,
		}
	}

	if result.IsLoggedIn {
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("账号 %s 当前已处于登录状态", accountID)}},
		}
	}

	now := time.Now()
	deadline := func() string {
		d, err := time.ParseDuration(result.Timeout)
		if err != nil {
			return now.Format("2006-01-02 15:04:05")
		}
		return now.Add(d).Format("2006-01-02 15:04:05")
	}()

	// 已登录：文本 + 图片
	contents := []MCPContent{
		{Type: "text", Text: fmt.Sprintf("请用小红书 App 在 %s 前扫码登录账号 %s 👇", deadline, accountID)},
		{
			Type:     "image",
			MimeType: "image/png",
			Data:     strings.TrimPrefix(result.Img, "data:image/png;base64,"),
		},
	}
	return &MCPToolResult{Content: contents}
}

// handlePublishContent 处理发布内容
func (s *AppServer) handlePublishContent(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 发布内容")

	title := stringFromArgs(args, "title")
	content := stringFromArgs(args, "content")
	imagePaths := stringSliceFromArgs(args, "images")
	tags := stringSliceFromArgs(args, "tags")

	if title == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布失败: 缺少title参数",
			}},
			IsError: true,
		}
	}
	if content == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布失败: 缺少content参数",
			}},
			IsError: true,
		}
	}
	if len(imagePaths) == 0 {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布失败: 缺少images参数",
			}},
			IsError: true,
		}
	}

	logrus.WithField("account", accountID).
		Infof("MCP: 发布内容 - 标题: %s, 图片数量: %d, 标签数量: %d", title, len(imagePaths), len(tags))

	// 构建发布请求
	req := &PublishRequest{
		Title:   title,
		Content: content,
		Images:  imagePaths,
		Tags:    tags,
	}

	// 执行发布
	result, err := s.xiaohongshuService.PublishContent(ctx, accountID, req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	resultText := fmt.Sprintf("内容发布成功: %+v", result)
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: resultText,
		}},
	}
}

// handlePublishVideo 处理发布视频内容
func (s *AppServer) handlePublishVideo(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 发布视频内容")

	title := stringFromArgs(args, "title")
	content := stringFromArgs(args, "content")
	video := stringFromArgs(args, "video")
	tags := stringSliceFromArgs(args, "tags")

	if title == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "发布视频失败: 缺少title参数"}}, IsError: true}
	}
	if content == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "发布视频失败: 缺少content参数"}}, IsError: true}
	}
	if video == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "发布视频失败: 缺少video参数"}}, IsError: true}
	}

	req := &PublishVideoRequest{
		Title:   title,
		Content: content,
		Video:   video,
		Tags:    tags,
	}

	result, err := s.xiaohongshuService.PublishVideo(ctx, accountID, req)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发布视频失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("发布视频成功，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(data),
		}},
	}
}

// handleListFeeds 处理获取账号推荐内容列表
func (s *AppServer) handleListFeeds(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 获取推荐内容列表")

	result, err := s.xiaohongshuService.ListFeeds(ctx, accountID)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取推荐内容列表失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 格式化输出，转换为JSON字符串
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("获取推荐内容列表成功，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

func (s *AppServer) handleListAccounts(ctx context.Context) *MCPToolResult {
	infos, err := accounts.ListAccounts()
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取账号列表失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	jsonData, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取账号列表成功，但序列化失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

func (s *AppServer) handleSetAccountRemark(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}
	remark := stringFromArgs(args, "remark")
	info, err := accounts.SetAccountRemark(accountID, remark)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "更新账号备注失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "更新账号备注成功，但序列化失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

func (s *AppServer) handleLikeFeed(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	feedID := stringFromArgs(args, "feed_id")
	if feedID == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "点赞失败: 缺少feed_id参数"}}, IsError: true}
	}
	xsecToken := stringFromArgs(args, "xsec_token")
	if xsecToken == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "点赞失败: 缺少xsec_token参数"}}, IsError: true}
	}
	unlike, _ := args["unlike"].(bool)

	logrus.WithField("account", accountID).
		Infof("MCP: 点赞操作 - Feed ID: %s, unlike: %v", feedID, unlike)

	var result *ActionResult
	if unlike {
		result, err = s.xiaohongshuService.UnlikeFeed(ctx, accountID, feedID, xsecToken)
	} else {
		result, err = s.xiaohongshuService.LikeFeed(ctx, accountID, feedID, xsecToken)
	}
	if err != nil {
		action := "点赞"
		if unlike {
			action = "取消点赞"
		}
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: action + "失败: " + err.Error()}}, IsError: true}
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("%s成功，但序列化失败: %v", result.Message, err)}}, IsError: true}
	}

	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(jsonData)}}}
}

func (s *AppServer) handleFavoriteFeed(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	feedID := stringFromArgs(args, "feed_id")
	if feedID == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "收藏失败: 缺少feed_id参数"}}, IsError: true}
	}
	xsecToken := stringFromArgs(args, "xsec_token")
	if xsecToken == "" {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "收藏失败: 缺少xsec_token参数"}}, IsError: true}
	}
	unfavorite, _ := args["unfavorite"].(bool)

	logrus.WithField("account", accountID).
		Infof("MCP: 收藏操作 - Feed ID: %s, unfavorite: %v", feedID, unfavorite)

	var result *ActionResult
	if unfavorite {
		result, err = s.xiaohongshuService.UnfavoriteFeed(ctx, accountID, feedID, xsecToken)
	} else {
		result, err = s.xiaohongshuService.FavoriteFeed(ctx, accountID, feedID, xsecToken)
	}
	if err != nil {
		action := "收藏"
		if unfavorite {
			action = "取消收藏"
		}
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: action + "失败: " + err.Error()}}, IsError: true}
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("%s成功，但序列化失败: %v", result.Message, err)}}, IsError: true}
	}

	return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: string(jsonData)}}}
}

// handleSearchFeeds 处理搜索Feeds
func (s *AppServer) handleSearchFeeds(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 搜索Feeds")

	// 解析参数
	keyword, ok := args["keyword"].(string)
	if !ok || keyword == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "搜索Feeds失败: 缺少关键词参数",
			}},
			IsError: true,
		}
	}

	logrus.WithField("account", accountID).Infof("MCP: 搜索Feeds - 关键词: %s", keyword)

	filters, err := xiaohongshu.NewSearchFilters(
		stringFromArgs(args, "sort"),
		stringFromArgs(args, "note_type"),
		stringFromArgs(args, "publish_time"),
		stringFromArgs(args, "search_scope"),
		stringFromArgs(args, "distance"),
	)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "搜索Feeds失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	result, err := s.xiaohongshuService.SearchFeeds(ctx, accountID, keyword, filters)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "搜索Feeds失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 格式化输出，转换为JSON字符串
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("搜索Feeds成功，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

// handleGetFeedDetail 处理获取Feed详情
func (s *AppServer) handleGetFeedDetail(ctx context.Context, args map[string]any) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 获取Feed详情")

	// 解析参数
	feedID, ok := args["feed_id"].(string)
	if !ok || feedID == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取Feed详情失败: 缺少feed_id参数",
			}},
			IsError: true,
		}
	}

	xsecToken, ok := args["xsec_token"].(string)
	if !ok || xsecToken == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取Feed详情失败: 缺少xsec_token参数",
			}},
			IsError: true,
		}
	}

	logrus.WithField("account", accountID).Infof("MCP: 获取Feed详情 - Feed ID: %s", feedID)

	result, err := s.xiaohongshuService.GetFeedDetail(ctx, accountID, feedID, xsecToken)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取Feed详情失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 格式化输出，转换为JSON字符串
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("获取Feed详情成功，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

// handleUserProfile 获取用户主页
func (s *AppServer) handleUserProfile(ctx context.Context, args map[string]any) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 获取用户主页")

	// 解析参数
	userID, ok := args["user_id"].(string)
	if !ok || userID == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取用户主页失败: 缺少user_id参数",
			}},
			IsError: true,
		}
	}

	xsecToken, ok := args["xsec_token"].(string)
	if !ok || xsecToken == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取用户主页失败: 缺少xsec_token参数",
			}},
			IsError: true,
		}
	}

	logrus.WithField("account", accountID).Infof("MCP: 获取用户主页 - User ID: %s", userID)

	result, err := s.xiaohongshuService.UserProfile(ctx, accountID, userID, xsecToken)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "获取用户主页失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 格式化输出，转换为JSON字符串
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("获取用户主页，但序列化失败: %v", err),
			}},
			IsError: true,
		}
	}

	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: string(jsonData),
		}},
	}
}

// handlePostComment 处理发表评论到Feed
func (s *AppServer) handlePostComment(ctx context.Context, args map[string]interface{}) *MCPToolResult {
	accountID, err := accountIDFromArgs(args)
	if err != nil {
		return accountErrorResult(err)
	}

	logrus.WithField("account", accountID).Info("MCP: 发表评论到Feed")

	// 解析参数
	feedID, ok := args["feed_id"].(string)
	if !ok || feedID == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发表评论失败: 缺少feed_id参数",
			}},
			IsError: true,
		}
	}

	xsecToken, ok := args["xsec_token"].(string)
	if !ok || xsecToken == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发表评论失败: 缺少xsec_token参数",
			}},
			IsError: true,
		}
	}

	content, ok := args["content"].(string)
	if !ok || content == "" {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发表评论失败: 缺少content参数",
			}},
			IsError: true,
		}
	}

	logrus.WithField("account", accountID).
		Infof("MCP: 发表评论 - Feed ID: %s, 内容长度: %d", feedID, len(content))

	// 发表评论
	result, err := s.xiaohongshuService.PostCommentToFeed(ctx, accountID, feedID, xsecToken, content)
	if err != nil {
		return &MCPToolResult{
			Content: []MCPContent{{
				Type: "text",
				Text: "发表评论失败: " + err.Error(),
			}},
			IsError: true,
		}
	}

	// 返回成功结果，只包含feed_id
	resultText := fmt.Sprintf("评论发表成功 - Feed ID: %s", result.FeedID)
	return &MCPToolResult{
		Content: []MCPContent{{
			Type: "text",
			Text: resultText,
		}},
	}
}
