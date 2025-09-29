package xiaohongshu

import (
	"context"
	"time"

	"github.com/go-rod/rod"
)

func waitForInitialState(page *rod.Page, expr string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			res, err := page.Evaluate(&rod.EvalOptions{JS: expr, ByValue: true})
			if err != nil {
				if err == context.Canceled {
					return err
				}
				continue
			}
			if res == nil || res.Value.Nil() {
				continue
			}
			if res.Value.Bool() {
				return nil
			}
		}
	}
}
