package xiaohongshu

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ActionResult 通用动作响应（点赞/收藏等）
type ActionResult struct {
	FeedID  string `json:"feed_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

const (
	selectorLikeButton    = ".interact-container .left .like-lottie"
	selectorCollectButton = ".interact-container .left .reds-icon.collect-icon"
)

type interactActionType string

const (
	actionLike       interactActionType = "点赞"
	actionFavorite   interactActionType = "收藏"
	actionUnlike     interactActionType = "取消点赞"
	actionUnfavorite interactActionType = "取消收藏"
)

type interactAction struct {
	page *rod.Page
}

func newInteractAction(page *rod.Page) *interactAction {
	return &interactAction{page: page}
}

func (a *interactAction) preparePage(ctx context.Context, actionType interactActionType, feedID, xsecToken string) (*rod.Page, error) {
	page := a.page.Context(ctx).Timeout(60 * time.Second)
	url := makeFeedDetailURL(feedID, xsecToken)
	logrus.Infof("Opening feed detail page for %s: %s", actionType, url)

	if err := page.Navigate(url); err != nil {
		return nil, err
	}
	page.MustWaitDOMStable()
	time.Sleep(1 * time.Second)

	return page, nil
}

func (a *interactAction) performClick(page *rod.Page, selector string) error {
	element, err := page.Element(selector)
	if err != nil {
		return err
	}
	if element == nil {
		return errors.Errorf("未找到操作按钮: %s", selector)
	}
	return element.Click(proto.InputMouseButtonLeft, 1)
}

type LikeAction struct {
	*interactAction
}

func NewLikeAction(page *rod.Page) *LikeAction {
	return &LikeAction{interactAction: newInteractAction(page)}
}

func (a *LikeAction) Like(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, true)
}

func (a *LikeAction) Unlike(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, false)
}

func (a *LikeAction) perform(ctx context.Context, feedID, xsecToken string, targetLiked bool) error {
	actionType := actionLike
	if !targetLiked {
		actionType = actionUnlike
	}

	page, err := a.preparePage(ctx, actionType, feedID, xsecToken)
	if err != nil {
		return err
	}

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
		return a.toggleLike(page, feedID, targetLiked, actionType)
	}

	if targetLiked && liked {
		logrus.Infof("feed %s already liked, skip clicking", feedID)
		return nil
	}
	if !targetLiked && !liked {
		logrus.Infof("feed %s not liked yet, skip clicking", feedID)
		return nil
	}

	return a.toggleLike(page, feedID, targetLiked, actionType)
}

func (a *LikeAction) toggleLike(page *rod.Page, feedID string, targetLiked bool, actionType interactActionType) error {
	if err := a.performClick(page, selectorLikeButton); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	liked, _, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if liked == targetLiked {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", feedID, actionType)
	if err := a.performClick(page, selectorLikeButton); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	liked, _, err = a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("第二次验证%s状态失败: %v", actionType, err)
		return nil
	}
	if liked == targetLiked {
		logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
		return nil
	}

	return nil
}

type FavoriteAction struct {
	*interactAction
}

func NewFavoriteAction(page *rod.Page) *FavoriteAction {
	return &FavoriteAction{interactAction: newInteractAction(page)}
}

func (a *FavoriteAction) Favorite(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, true)
}

func (a *FavoriteAction) Unfavorite(ctx context.Context, feedID, xsecToken string) error {
	return a.perform(ctx, feedID, xsecToken, false)
}

func (a *FavoriteAction) perform(ctx context.Context, feedID, xsecToken string, targetCollected bool) error {
	actionType := actionFavorite
	if !targetCollected {
		actionType = actionUnfavorite
	}

	page, err := a.preparePage(ctx, actionType, feedID, xsecToken)
	if err != nil {
		return err
	}

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("failed to read interact state: %v (continue to try clicking)", err)
		return a.toggleFavorite(page, feedID, targetCollected, actionType)
	}

	if targetCollected && collected {
		logrus.Infof("feed %s already favorited, skip clicking", feedID)
		return nil
	}
	if !targetCollected && !collected {
		logrus.Infof("feed %s not favorited yet, skip clicking", feedID)
		return nil
	}

	return a.toggleFavorite(page, feedID, targetCollected, actionType)
}

func (a *FavoriteAction) toggleFavorite(page *rod.Page, feedID string, targetCollected bool, actionType interactActionType) error {
	if err := a.performClick(page, selectorCollectButton); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)

	_, collected, err := a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("验证%s状态失败: %v", actionType, err)
		return nil
	}
	if collected == targetCollected {
		logrus.Infof("feed %s %s成功", feedID, actionType)
		return nil
	}

	logrus.Warnf("feed %s %s可能未成功，状态未变化，尝试再次点击", feedID, actionType)
	if err := a.performClick(page, selectorCollectButton); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	_, collected, err = a.getInteractState(page, feedID)
	if err != nil {
		logrus.Warnf("第二次验证%s状态失败: %v", actionType, err)
		return nil
	}
	if collected == targetCollected {
		logrus.Infof("feed %s 第二次点击%s成功", feedID, actionType)
		return nil
	}

	return nil
}

func (a *interactAction) getInteractState(page *rod.Page, feedID string) (liked bool, collected bool, err error) {
	result := page.MustEval(`() => {
        if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.note && window.__INITIAL_STATE__.note.noteDetailMap) {
            return JSON.stringify(window.__INITIAL_STATE__.note.noteDetailMap);
        }
        return "";
    }`).Str()

	if result == "" {
		return false, false, errors.New("__INITIAL_STATE__ not found")
	}

	var noteDetailMap map[string]struct {
		Note struct {
			InteractInfo struct {
				Liked     bool `json:"liked"`
				Collected bool `json:"collected"`
			} `json:"interactInfo"`
		} `json:"note"`
	}

	if err := json.Unmarshal([]byte(result), &noteDetailMap); err != nil {
		return false, false, errors.Wrap(err, "unmarshal note detail map failed")
	}

	noteDetail, ok := noteDetailMap[feedID]
	if !ok {
		return false, false, fmt.Errorf("feed %s not found in note detail map", feedID)
	}

	return noteDetail.Note.InteractInfo.Liked, noteDetail.Note.InteractInfo.Collected, nil
}
