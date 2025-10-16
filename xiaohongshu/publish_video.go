package xiaohongshu

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishVideoContent 发布视频内容
type PublishVideoContent struct {
	Title     string
	Content   string
	Tags      []string
	VideoPath string
}

// NewPublishVideoAction 进入发布页并切换到“上传视频”
func NewPublishVideoAction(page *rod.Page) (*PublishAction, error) {
	pp := page.Timeout(90 * time.Second)

	pp.MustNavigate(urlOfPublic)

	if err := waitPublishEditorReady(pp); err != nil {
		return nil, err
	}

	if err := clickPublishTab(pp, "上传视频"); err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{page: pp}, nil
}

// PublishVideo 上传视频并提交
func (p *PublishAction) PublishVideo(ctx context.Context, content PublishVideoContent) error {
	if strings.TrimSpace(content.VideoPath) == "" {
		return errors.New("视频不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadVideo(page, content.VideoPath); err != nil {
		return errors.Wrap(err, "小红书上传视频失败")
	}

	if err := submitPublishVideo(page, content.Title, content.Content, content.Tags); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}
	return nil
}

// uploadVideo 上传单个本地视频
func uploadVideo(page *rod.Page, videoPath string) error {
	pp := page.Timeout(5 * time.Minute)

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return errors.Wrapf(err, "视频文件不存在: %s", videoPath)
	}

	fileInput, err := pp.Element(".upload-input")
	if err != nil || fileInput == nil {
		fileInput, err = pp.Element("input[type='file']")
		if err != nil || fileInput == nil {
			return errors.New("未找到视频上传输入框")
		}
	}

	if err := fileInput.SetFiles([]string{videoPath}); err != nil {
		return errors.Wrap(err, "视频文件选择失败")
	}

	btn, err := waitForPublishButtonClickable(pp)
	if err != nil {
		return err
	}
	slog.Info("视频上传/处理完成，发布按钮可点击", "button", btn)
	return nil
}

// waitForPublishButtonClickable 等待发布按钮可点击
func waitForPublishButtonClickable(page *rod.Page) (*rod.Element, error) {
	maxWait := 10 * time.Minute
	interval := 1 * time.Second
	start := time.Now()
	selector := "button.publishBtn"

	slog.Info("开始等待发布按钮可点击(视频)")

	for time.Since(start) < maxWait {
		btn, err := page.Element(selector)
		if err == nil && btn != nil {
			vis, verr := btn.Visible()
			if verr == nil && vis {
				if disabled, _ := btn.Attribute("disabled"); disabled == nil {
					if cls, _ := btn.Attribute("class"); cls == nil || !strings.Contains(*cls, "disabled") {
						return btn, nil
					}
					return btn, nil
				}
			}
		}
		time.Sleep(interval)
	}
	return nil, errors.New("等待发布按钮可点击超时")
}

// submitPublishVideo 填写标题、正文、标签并点击发布
func submitPublishVideo(page *rod.Page, title, content string, tags []string) error {
	titleElem, err := page.Element("div.d-input input")
	if err != nil {
		return errors.Wrap(err, "未找到标题输入框")
	}
	if titleElem == nil {
		return errors.New("标题输入框为空")
	}
	if err := titleElem.Input(title); err != nil {
		return errors.Wrap(err, "标题输入失败")
	}
	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		if err := contentElem.Input(content); err != nil {
			return errors.Wrap(err, "正文输入失败")
		}
		inputTags(contentElem, tags)
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	btn, err := waitForPublishButtonClickable(page)
	if err != nil {
		return err
	}

	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击发布按钮失败")
	}

	time.Sleep(3 * time.Second)
	return nil
}
