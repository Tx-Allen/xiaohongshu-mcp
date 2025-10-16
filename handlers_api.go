package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/accounts"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

// respondError 返回错误响应
func respondError(c *gin.Context, statusCode int, code, message string, details any) {
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}

	logrus.Errorf("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), statusCode)

	c.JSON(statusCode, response)
}

// respondSuccess 返回成功响应
func respondSuccess(c *gin.Context, data any, message string) {
	response := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	logrus.Infof("%s %s %s %d", c.Request.Method, c.Request.URL.Path,
		c.GetString("account"), http.StatusOK)

	c.JSON(http.StatusOK, response)
}

func resolveAccountID(c *gin.Context, raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		respondError(c, http.StatusBadRequest, "MISSING_ACCOUNT_ID",
			"缺少账号参数", "account_id is required")
		return "", false
	}

	resolved, err := accounts.ResolveAccountID(trimmed)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ACCOUNT_ID",
			"账号格式不正确", err.Error())
		return "", false
	}

	return resolved, true
}

func accountIDFromQuery(c *gin.Context) (string, bool) {
	return resolveAccountID(c, c.Query("account_id"))
}

// checkLoginStatusHandler 检查登录状态
func (s *AppServer) checkLoginStatusHandler(c *gin.Context) {
	accountID, ok := accountIDFromQuery(c)
	if !ok {
		return
	}

	status, err := s.xiaohongshuService.CheckLoginStatus(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED",
			"检查登录状态失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, status, "检查登录状态成功")
}

// getLoginQrcodeHandler 处理 [GET /api/login/qrcode] 请求。
// 用于生成并返回登录二维码（Base64 图片 + 超时时间），供前端展示给用户扫码登录。
func (s *AppServer) getLoginQrcodeHandler(c *gin.Context) {
	accountID, ok := accountIDFromQuery(c)
	if !ok {
		return
	}

	result, err := s.xiaohongshuService.GetLoginQrcode(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "STATUS_CHECK_FAILED",
			"获取登录二维码失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "获取登录二维码成功")
}

// publishHandler 发布内容
func (s *AppServer) publishHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		PublishRequest
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	accountID, ok := resolveAccountID(c, payload.AccountID)
	if !ok {
		return
	}

	// 执行发布
	result, err := s.xiaohongshuService.PublishContent(c.Request.Context(), accountID, &payload.PublishRequest)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_FAILED",
			"发布失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "发布成功")
}

// publishVideoHandler 发布视频内容
func (s *AppServer) publishVideoHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		PublishVideoRequest
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	accountID, ok := resolveAccountID(c, payload.AccountID)
	if !ok {
		return
	}

	result, err := s.xiaohongshuService.PublishVideo(c.Request.Context(), accountID, &payload.PublishVideoRequest)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PUBLISH_VIDEO_FAILED",
			"发布视频失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "发布视频成功")
}

// listFeedsHandler 获取账号推荐内容列表
func (s *AppServer) listFeedsHandler(c *gin.Context) {
	accountID, ok := accountIDFromQuery(c)
	if !ok {
		return
	}
	// 获取 Feeds 列表
	result, err := s.xiaohongshuService.ListFeeds(c.Request.Context(), accountID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_FEEDS_FAILED",
			"获取推荐内容列表失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "获取推荐内容列表成功")
}

// searchFeedsHandler 搜索Feeds
func (s *AppServer) searchFeedsHandler(c *gin.Context) {
	accountID, ok := accountIDFromQuery(c)
	if !ok {
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		respondError(c, http.StatusBadRequest, "MISSING_KEYWORD",
			"缺少关键词参数", "keyword parameter is required")
		return
	}

	filters, err := xiaohongshu.NewSearchFilters(
		strings.TrimSpace(c.Query("sort")),
		strings.TrimSpace(c.Query("note_type")),
		strings.TrimSpace(c.Query("publish_time")),
		strings.TrimSpace(c.Query("search_scope")),
		strings.TrimSpace(c.Query("distance")),
	)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_FILTER",
			"筛选参数不合法", err.Error())
		return
	}

	// 搜索 Feeds
	result, err := s.xiaohongshuService.SearchFeeds(c.Request.Context(), accountID, keyword, filters)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SEARCH_FEEDS_FAILED",
			"搜索Feeds失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "搜索Feeds成功")
}

// getFeedDetailHandler 获取Feed详情
func (s *AppServer) getFeedDetailHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		FeedDetailRequest
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	accountID, ok := resolveAccountID(c, payload.AccountID)
	if !ok {
		return
	}

	// 获取 Feed 详情
	result, err := s.xiaohongshuService.GetFeedDetail(c.Request.Context(), accountID, payload.FeedID, payload.XsecToken)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_FEED_DETAIL_FAILED",
			"获取Feed详情失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, "获取Feed详情成功")
}

// userProfileHandler 用户主页
func (s *AppServer) userProfileHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		UserProfileRequest
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	accountID, ok := resolveAccountID(c, payload.AccountID)
	if !ok {
		return
	}

	// 获取用户信息
	result, err := s.xiaohongshuService.UserProfile(c.Request.Context(), accountID, payload.UserID, payload.XsecToken)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GET_USER_PROFILE_FAILED",
			"获取用户主页失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, map[string]any{"data": result}, "result.Message")
}

// postCommentHandler 发表评论到Feed
func (s *AppServer) postCommentHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		PostCommentRequest
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	accountID, ok := resolveAccountID(c, payload.AccountID)
	if !ok {
		return
	}

	// 发表评论
	result, err := s.xiaohongshuService.PostCommentToFeed(c.Request.Context(), accountID, payload.FeedID, payload.XsecToken, payload.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "POST_COMMENT_FAILED",
			"发表评论失败", err.Error())
		return
	}

	c.Set("account", accountID)
	respondSuccess(c, result, result.Message)
}

// healthHandler 健康检查
func healthHandler(c *gin.Context) {
	respondSuccess(c, map[string]any{
		"status":    "healthy",
		"service":   "xiaohongshu-mcp",
		"account":   "ai-report",
		"timestamp": "now",
	}, "服务正常")
}

// listAccountsHandler 返回所有账号信息
func (s *AppServer) listAccountsHandler(c *gin.Context) {
	infos, err := accounts.ListAccounts()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "LIST_ACCOUNTS_FAILED",
			"获取账号列表失败", err.Error())
		return
	}

	c.Set("account", "*")
	respondSuccess(c, map[string]any{"accounts": infos}, "获取账号列表成功")
}

// setAccountRemarkHandler 更新账号备注
func (s *AppServer) setAccountRemarkHandler(c *gin.Context) {
	var payload struct {
		AccountID string `json:"account_id" binding:"required"`
		Remark    string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
			"请求参数错误", err.Error())
		return
	}

	info, err := accounts.SetAccountRemark(payload.AccountID, payload.Remark)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "SET_ACCOUNT_REMARK_FAILED",
			"更新账号备注失败", err.Error())
		return
	}

	c.Set("account", info.ID)
	respondSuccess(c, info, "更新账号备注成功")
}
