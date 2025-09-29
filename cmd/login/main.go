package main

import (
	"context"
	"encoding/json"
	"flag"
	"strings"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/accounts"
	"github.com/xpzouying/xiaohongshu-mcp/browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

func main() {
	var (
		binPath   string // 浏览器二进制文件路径
		accountID string // 账号标识
	)
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.StringVar(&accountID, "account", "", "账号标识，用于区分 cookies 存储")
	flag.Parse()

	resolvedAccountID, err := accounts.ResolveAccountID(accountID)
	if err != nil {
		logrus.Fatalf("invalid account id: %v", err)
	}

	if strings.TrimSpace(accountID) == "" {
		logrus.Infof("未指定账号，使用默认账号 %s。若需多账号，请添加 --account 标识，例如 --account brand_a", resolvedAccountID)
	} else {
		logrus.Infof("即将登录账号: %s", resolvedAccountID)
	}

	cookiePath, err := accounts.CookiesPath(resolvedAccountID)
	if err != nil {
		logrus.Fatalf("failed to resolve cookies path: %v", err)
	}

	// 登录的时候，需要界面，所以不能无头模式
	options := []browser.Option{browser.WithCookiesPath(cookiePath)}
	if binPath != "" {
		options = append(options, browser.WithBinPath(binPath))
	}

	b := browser.NewBrowser(false, options...)
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewLogin(page)

	status, err := action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status: %v", err)
	}

	logrus.Infof("当前登录状态: %v", status)

	if status {
		return
	}

	// 开始登录流程
	logrus.Info("开始登录流程...")
	if err = action.Login(context.Background()); err != nil {
		logrus.Fatalf("登录失败: %v", err)
	} else {
		if err := saveCookies(resolvedAccountID, page); err != nil {
			logrus.Fatalf("failed to save cookies: %v", err)
		}
	}

	// 再次检查登录状态确认成功
	status, err = action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status after login: %v", err)
	}

	if status {
		logrus.Infof("账号 %s 登录成功！", resolvedAccountID)
	} else {
		logrus.Errorf("账号 %s 登录流程完成但仍未登录", resolvedAccountID)
	}

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
