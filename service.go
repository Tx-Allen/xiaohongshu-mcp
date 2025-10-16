package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/mattn/go-runewidth"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/accounts"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/pkg/downloader"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// XiaohongshuService 小红书业务服务
type XiaohongshuService struct{}

// NewXiaohongshuService 创建小红书服务实例
func NewXiaohongshuService() *XiaohongshuService {
	return &XiaohongshuService{}
}

// PublishRequest 发布请求
type PublishRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Images  []string `json:"images" binding:"required,min=1"`
	Tags    []string `json:"tags,omitempty"`
}

// LoginStatusResponse 登录状态响应
type LoginStatusResponse struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Username   string `json:"username,omitempty"`
}

// LoginQrcodeResponse 登录扫码二维码
type LoginQrcodeResponse struct {
	Timeout    string `json:"timeout"`
	IsLoggedIn bool   `json:"is_logged_in"`
	Img        string `json:"img,omitempty"`
}

// PublishResponse 发布响应
type PublishResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Images  int    `json:"images"`
	Status  string `json:"status"`
	PostID  string `json:"post_id,omitempty"`
}

// PublishVideoRequest 发布视频请求（仅支持本地单个视频文件）
type PublishVideoRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Video   string   `json:"video" binding:"required"`
	Tags    []string `json:"tags,omitempty"`
}

// PublishVideoResponse 发布视频响应
type PublishVideoResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Video   string `json:"video"`
	Status  string `json:"status"`
	PostID  string `json:"post_id,omitempty"`
}

// ActionResult 通用操作响应
type ActionResult struct {
	FeedID  string `json:"feed_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// FeedsListResponse Feeds列表响应
type FeedsListResponse struct {
	Feeds []xiaohongshu.Feed `json:"feeds"`
	Count int                `json:"count"`
}

// UserProfileResponse 用户主页响应
type UserProfileResponse struct {
	UserBasicInfo xiaohongshu.UserBasicInfo      `json:"userBasicInfo"`
	Interactions  []xiaohongshu.UserInteractions `json:"interactions"`
	Feeds         []xiaohongshu.Feed             `json:"feeds"`
}

// CheckLoginStatus 检查登录状态
func (s *XiaohongshuService) CheckLoginStatus(ctx context.Context, accountID string) (*LoginStatusResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	loginAction := xiaohongshu.NewLogin(page)

	isLoggedIn, err := loginAction.CheckLoginStatus(ctx)
	if err != nil {
		return nil, err
	}

	response := &LoginStatusResponse{
		IsLoggedIn: isLoggedIn,
		Username:   configs.Username,
	}

	return response, nil
}

// GetLoginQrcode 获取登录的扫码二维码
func (s *XiaohongshuService) GetLoginQrcode(ctx context.Context, accountID string) (*LoginQrcodeResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	page := b.NewPage()

	deferFunc := func() {
		_ = page.Close()
		b.Close()
	}

	loginAction := xiaohongshu.NewLogin(page)

	img, loggedIn, err := loginAction.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		defer deferFunc()
	}
	if err != nil {
		return nil, err
	}

	timeout := 4 * time.Minute

	if !loggedIn {
		go func(account string) {
			ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			defer deferFunc()

			if loginAction.WaitForLogin(ctxTimeout) {
				if er := saveCookies(account, page); er != nil {
					logrus.Errorf("failed to save cookies for account %s: %v", account, er)
				}
			}
		}(accountID)
	}

	return &LoginQrcodeResponse{
		Timeout: func() string {
			if loggedIn {
				return "0s"
			}
			return timeout.String()
		}(),
		Img:        img,
		IsLoggedIn: loggedIn,
	}, nil
}

// PublishContent 发布内容
func (s *XiaohongshuService) PublishContent(ctx context.Context, accountID string, req *PublishRequest) (*PublishResponse, error) {
	// 验证标题长度
	// 小红书限制：最大40个单位长度
	// 中文/日文/韩文占2个单位，英文/数字占1个单位
	if titleWidth := runewidth.StringWidth(req.Title); titleWidth > 40 {
		return nil, fmt.Errorf("标题长度超过限制")
	}

	// 处理图片：下载URL图片或使用本地路径
	imagePaths, err := s.processImages(accountID, req.Images)
	if err != nil {
		return nil, err
	}

	// 构建发布内容
	content := xiaohongshu.PublishImageContent{
		Title:      req.Title,
		Content:    req.Content,
		Tags:       req.Tags,
		ImagePaths: imagePaths,
	}

	// 执行发布
	if err := s.publishContent(ctx, accountID, content); err != nil {
		return nil, err
	}

	response := &PublishResponse{
		Title:   req.Title,
		Content: req.Content,
		Images:  len(imagePaths),
		Status:  "发布完成",
	}

	return response, nil
}

// PublishVideo 发布视频内容
func (s *XiaohongshuService) PublishVideo(ctx context.Context, accountID string, req *PublishVideoRequest) (*PublishVideoResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action, err := xiaohongshu.NewPublishVideoAction(page)
	if err != nil {
		return nil, err
	}

	content := xiaohongshu.PublishVideoContent{
		Title:     req.Title,
		Content:   req.Content,
		Tags:      req.Tags,
		VideoPath: req.Video,
	}

	if err := action.PublishVideo(ctx, content); err != nil {
		return nil, err
	}

	response := &PublishVideoResponse{
		Title:   req.Title,
		Content: req.Content,
		Video:   req.Video,
		Status:  "发布完成",
	}

	return response, nil
}

// processImages 处理图片列表，支持URL下载和本地路径
func (s *XiaohongshuService) processImages(accountID string, images []string) ([]string, error) {
	imageDir, err := accounts.ImagesDir(accountID)
	if err != nil {
		return nil, err
	}

	processor := downloader.NewImageProcessor(imageDir)
	return processor.ProcessImages(images)
}

// publishContent 执行内容发布
func (s *XiaohongshuService) publishContent(ctx context.Context, accountID string, content xiaohongshu.PublishImageContent) error {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action, err := xiaohongshu.NewPublishImageAction(page)
	if err != nil {
		return err
	}

	// 执行发布
	return action.Publish(ctx, content)
}

// LikeFeed 点赞笔记
func (s *XiaohongshuService) LikeFeed(ctx context.Context, accountID, feedID, xsecToken string) (*ActionResult, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewLikeAction(page)
	if err := action.Like(ctx, feedID, xsecToken); err != nil {
		return nil, err
	}

	return &ActionResult{FeedID: feedID, Success: true, Message: "点赞成功或已点赞"}, nil
}

// UnlikeFeed 取消点赞
func (s *XiaohongshuService) UnlikeFeed(ctx context.Context, accountID, feedID, xsecToken string) (*ActionResult, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewLikeAction(page)
	if err := action.Unlike(ctx, feedID, xsecToken); err != nil {
		return nil, err
	}

	return &ActionResult{FeedID: feedID, Success: true, Message: "取消点赞成功或未点赞"}, nil
}

// FavoriteFeed 收藏笔记
func (s *XiaohongshuService) FavoriteFeed(ctx context.Context, accountID, feedID, xsecToken string) (*ActionResult, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewFavoriteAction(page)
	if err := action.Favorite(ctx, feedID, xsecToken); err != nil {
		return nil, err
	}

	return &ActionResult{FeedID: feedID, Success: true, Message: "收藏成功或已收藏"}, nil
}

// UnfavoriteFeed 取消收藏
func (s *XiaohongshuService) UnfavoriteFeed(ctx context.Context, accountID, feedID, xsecToken string) (*ActionResult, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewFavoriteAction(page)
	if err := action.Unfavorite(ctx, feedID, xsecToken); err != nil {
		return nil, err
	}

	return &ActionResult{FeedID: feedID, Success: true, Message: "取消收藏成功或未收藏"}, nil
}

// ListFeeds 获取指定账号的推荐内容列表
func (s *XiaohongshuService) ListFeeds(ctx context.Context, accountID string) (*FeedsListResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// 创建 Feeds 列表 action
	action, err := xiaohongshu.NewFeedsListAction(page)
	if err != nil {
		return nil, err
	}

	// 获取 Feeds 列表
	feeds, err := action.GetFeedsList(ctx)
	if err != nil {
		return nil, err
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

func (s *XiaohongshuService) SearchFeeds(ctx context.Context, accountID, keyword string, filters *xiaohongshu.SearchFilters) (*FeedsListResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewSearchAction(page)

	feeds, err := action.Search(ctx, keyword, filters)
	if err != nil {
		return nil, err
	}

	response := &FeedsListResponse{
		Feeds: feeds,
		Count: len(feeds),
	}

	return response, nil
}

// GetFeedDetail 获取Feed详情
func (s *XiaohongshuService) GetFeedDetail(ctx context.Context, accountID, feedID, xsecToken string) (*FeedDetailResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// 创建 Feed 详情 action
	action := xiaohongshu.NewFeedDetailAction(page)

	// 获取 Feed 详情
	result, err := action.GetFeedDetail(ctx, feedID, xsecToken)
	if err != nil {
		return nil, err
	}

	response := &FeedDetailResponse{
		FeedID: feedID,
		Data:   result,
	}

	return response, nil
}

// UserProfile 获取用户信息
func (s *XiaohongshuService) UserProfile(ctx context.Context, accountID, userID, xsecToken string) (*UserProfileResponse, error) {
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewUserProfileAction(page)

	result, err := action.UserProfile(ctx, userID, xsecToken)
	if err != nil {
		return nil, err
	}
	response := &UserProfileResponse{
		UserBasicInfo: result.UserBasicInfo,
		Interactions:  result.Interactions,
		Feeds:         result.Feeds,
	}

	return response, nil

}

// PostCommentToFeed 发表评论到Feed
func (s *XiaohongshuService) PostCommentToFeed(ctx context.Context, accountID, feedID, xsecToken, content string) (*PostCommentResponse, error) {
	// 使用非无头模式以便查看操作过程
	b, err := s.newBrowser(accountID)
	if err != nil {
		return nil, err
	}
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	// 创建 Feed 评论 action
	action := xiaohongshu.NewCommentFeedAction(page)

	// 发表评论
	if err := action.PostComment(ctx, feedID, xsecToken, content); err != nil {
		return nil, err
	}

	response := &PostCommentResponse{
		FeedID:  feedID,
		Success: true,
		Message: "评论发表成功",
	}

	return response, nil
}

func (s *XiaohongshuService) newBrowser(accountID string) (*headless_browser.Browser, error) {
	cookiePath, err := accounts.CookiesPath(accountID)
	if err != nil {
		return nil, err
	}

	opts := []browser.Option{
		browser.WithCookiesPath(cookiePath),
	}

	if bin := configs.GetBinPath(); bin != "" {
		opts = append(opts, browser.WithBinPath(bin))
	}

	return browser.NewBrowser(configs.IsHeadless(), opts...), nil
}

func saveCookies(accountID string, page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookiePath, err := accounts.CookiesPath(accountID)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookiePath)
	return cookieLoader.SaveCookies(data)
}
