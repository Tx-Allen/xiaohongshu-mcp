package browser

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/cookies"
)

type browserConfig struct {
	binPath     string
	cookiesPath string
}

type Option func(*browserConfig)

func WithBinPath(binPath string) Option {
	return func(c *browserConfig) {
		c.binPath = binPath
	}
}

func WithCookiesPath(path string) Option {
	return func(c *browserConfig) {
		c.cookiesPath = path
	}
}

func NewBrowser(headless bool, options ...Option) *headless_browser.Browser {
	cfg := &browserConfig{}
	for _, opt := range options {
		opt(cfg)
	}

	opts := []headless_browser.Option{
		headless_browser.WithHeadless(headless),
	}
	if cfg.binPath != "" {
		opts = append(opts, headless_browser.WithChromeBinPath(cfg.binPath))
	}

	// 加载 cookies
	cookiePath := cfg.cookiesPath
	if cookiePath == "" {
		cookiePath = cookies.GetCookiesFilePath()
	}

	if cookiePath != "" {
		if infoErr := ensureCookieAvailability(cookiePath); infoErr != nil {
			logrus.Warnf("failed to inspect cookies path %s: %v", cookiePath, infoErr)
		} else if _, err := os.Stat(cookiePath); err == nil {
			cookieLoader := cookies.NewLoadCookie(cookiePath)
			if data, loadErr := cookieLoader.LoadCookies(); loadErr == nil {
				opts = append(opts, headless_browser.WithCookies(string(data)))
				logrus.Debugf("loaded cookies from file: %s", cookiePath)
			} else {
				logrus.Warnf("failed to load cookies from %s: %v", cookiePath, loadErr)
			}
		} else if !os.IsNotExist(err) {
			logrus.Warnf("failed to stat cookies file %s: %v", cookiePath, err)
		}
	}

	return headless_browser.New(opts...)
}

func ensureCookieAvailability(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
