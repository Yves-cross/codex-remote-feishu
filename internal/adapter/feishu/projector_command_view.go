package feishu

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func commandPageViewFromView(view control.FeishuCommandView, ctx *control.FeishuUICommandContext) (control.FeishuCommandPageView, bool) {
	productMode := ""
	stage := ""
	if ctx != nil {
		productMode = strings.TrimSpace(ctx.Surface.ProductMode)
		stage = strings.TrimSpace(ctx.MenuStage)
	}
	return control.FeishuCommandPageViewFromView(view, productMode, stage)
}
