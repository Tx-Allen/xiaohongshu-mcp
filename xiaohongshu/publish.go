package xiaohongshu

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	Tags       []string
	ImagePaths []string
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(90 * time.Second)

	pp.MustNavigate(urlOfPublic)

	if err := waitPublishEditorReady(pp); err != nil {
		return nil, err
	}

	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	if err := clickPublishTab(pp, "上传图文"); err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{
		page: pp,
	}, nil
}

func (p *PublishAction) Publish(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content, content.Tags); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func clickPublishTab(page *rod.Page, label string) error {
	createElems, err := page.Elements("div.creator-tab")
	if err != nil {
		return err
	}

	var visibleElems []*rod.Element
	for _, elem := range createElems {
		if isElementVisible(elem) {
			visibleElems = append(visibleElems, elem)
		}
	}

	if len(visibleElems) == 0 {
		return errors.New("没有找到上传元素")
	}

	for _, elem := range visibleElems {
		text, err := elem.Text()
		if err != nil {
			slog.Error("获取元素文本失败", "error", err)
			continue
		}

		if text == label {
			if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Error("点击发布TAB失败", "label", label, "error", err)
				continue
			}
			return nil
		}
	}

	return errors.Errorf("未找到发布TAB: %s", label)
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	// 验证文件路径有效性
	for _, path := range imagesPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Wrapf(err, "图片文件不存在: %s", path)
		}
	}

	// 等待上传输入框出现
	uploadInput, err := pp.Element(".upload-input")
	if err != nil {
		return err
	}
	if uploadInput == nil {
		return errors.New("未找到图片上传输入框")
	}

	// 上传多个文件
	if err := uploadInput.SetFiles(imagesPaths); err != nil {
		return errors.Wrap(err, "设置上传文件失败")
	}

	// 等待并验证上传完成
	return waitForUploadComplete(pp, len(imagesPaths))
}

// waitForUploadComplete 等待并验证上传完成
func waitForUploadComplete(page *rod.Page, expectedCount int) error {
	maxWaitTime := 90 * time.Second
	checkInterval := 500 * time.Millisecond
	start := time.Now()

	slog.Info("开始等待图片上传完成", "expected_count", expectedCount)

	for time.Since(start) < maxWaitTime {
		// 使用具体的pr类名检查已上传的图片
		uploadedImages, err := page.Elements(".img-preview-area .pr")

		slog.Info("uploadedImages", "uploadedImages", uploadedImages)

		if err == nil {
			currentCount := len(uploadedImages)
			slog.Info("检测到已上传图片", "current_count", currentCount, "expected_count", expectedCount)
			if currentCount >= expectedCount {
				slog.Info("所有图片上传完成", "count", currentCount)
				return nil
			}
		} else {
			slog.Debug("未找到已上传图片元素")
		}

		time.Sleep(checkInterval)
	}

	return errors.New("上传超时，请检查网络连接和图片大小")
}

func waitPublishEditorReady(page *rod.Page) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		el, err := page.Element("div.upload-content")
		if err == nil && el != nil {
			visible, visErr := el.Visible()
			if visErr == nil && visible {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("发布编辑器未在预期时间内准备就绪")
}

func submitPublish(page *rod.Page, title, content string, tags []string) error {

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

	submitButton, err := page.Element("div.submit div.d-button-content")
	if err != nil {
		return errors.Wrap(err, "未找到提交按钮")
	}
	if submitButton == nil {
		return errors.New("提交按钮为空")
	}
	if err := submitButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击提交按钮失败")
	}

	time.Sleep(3 * time.Second)

	return nil
}

// 查找内容输入框 - 使用Race方法处理两种样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	var foundElement *rod.Element
	var found bool

	page.Race().
		Element("div.ql-editor").MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		ElementFunc(func(page *rod.Page) (*rod.Element, error) {
			return findTextboxByPlaceholder(page)
		}).MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		MustDo()

	if found {
		return foundElement, true
	}

	slog.Warn("no content element found by any method")
	return nil, false
}

func inputTags(contentElem *rod.Element, tags []string) {
	if len(tags) == 0 {
		return
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < 20; i++ {
		contentElem.MustKeyActions().
			Type(input.ArrowDown).
			MustDo()
		time.Sleep(10 * time.Millisecond)
	}

	contentElem.MustKeyActions().
		Press(input.Enter).
		Press(input.Enter).
		MustDo()

	time.Sleep(1 * time.Second)

	for _, tag := range tags {
		tag = strings.TrimLeft(tag, "#")
		inputTag(contentElem, tag)
	}
}

func inputTag(contentElem *rod.Element, tag string) {
	contentElem.MustInput("#")
	time.Sleep(200 * time.Millisecond)

	for _, char := range tag {
		contentElem.MustInput(string(char))
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	page := contentElem.Page()
	topicContainer, err := page.Element("#creator-editor-topic-container")
	if err == nil && topicContainer != nil {
		firstItem, err := topicContainer.Element(".item")
		if err == nil && firstItem != nil {
			firstItem.MustClick()
			slog.Info("成功点击标签联想选项", "tag", tag)
			time.Sleep(200 * time.Millisecond)
		} else {
			slog.Warn("未找到标签联想选项，直接输入空格", "tag", tag)
			// 如果没有找到联想选项，输入空格结束
			contentElem.MustInput(" ")
		}
	} else {
		slog.Warn("未找到标签联想下拉框，直接输入空格", "tag", tag)
		// 如果没有找到下拉框，输入空格结束
		contentElem.MustInput(" ")
	}

	time.Sleep(500 * time.Millisecond) // 等待标签处理完成
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	// 查找包含指定placeholder的元素
	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	// 向上查找textbox父元素
	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

func findPlaceholderElement(elements []*rod.Element, searchText string) *rod.Element {
	for _, elem := range elements {
		placeholder, err := elem.Attribute("data-placeholder")
		if err != nil || placeholder == nil {
			continue
		}

		if strings.Contains(*placeholder, searchText) {
			return elem
		}
	}
	return nil
}

func findTextboxParent(elem *rod.Element) *rod.Element {
	currentElem := elem
	for i := 0; i < 5; i++ {
		parent, err := currentElem.Parent()
		if err != nil {
			break
		}

		role, err := parent.Attribute("role")
		if err != nil || role == nil {
			currentElem = parent
			continue
		}

		if *role == "textbox" {
			return parent
		}

		currentElem = parent
	}
	return nil
}

// isElementVisible 检查元素是否可见
func isElementVisible(elem *rod.Element) bool {

	// 检查是否有隐藏样式
	style, err := elem.Attribute("style")
	if err == nil && style != nil {
		styleStr := *style

		if strings.Contains(styleStr, "left: -9999px") ||
			strings.Contains(styleStr, "top: -9999px") ||
			strings.Contains(styleStr, "position: absolute; left: -9999px") ||
			strings.Contains(styleStr, "display: none") ||
			strings.Contains(styleStr, "visibility: hidden") {
			return false
		}
	}

	visible, err := elem.Visible()
	if err != nil {
		slog.Warn("无法获取元素可见性", "error", err)
		return true
	}

	return visible
}
