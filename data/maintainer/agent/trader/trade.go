package main

import (
	"fmt"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// TradeRun is the single orchestrator custom action for the daily arbitrage
// task. One invocation runs the full flow:
//
//	load calendar → print today's sell plan → navigate to merchant (pipeline) →
//	sell peak items → bargain + buy all favorites → return home (pipeline)
//
// Selling is calendar-driven (today's peak-price items). Buying is
// favorites-driven: the user stars the items they want to stock up on, and
// the agent clicks "购买全部收藏".
//
// Static UI navigation lives in the pipeline (Trade_EnterMerchant,
// Trade_ReturnHome) and is driven via ctx.RunTask; the data-driven sell/buy
// loops run in this agent using direct recognitions/actions.
type TradeRun struct{}

var _ maa.CustomActionRunner = &TradeRun{}

// Run implements maa.CustomActionRunner.
func (a *TradeRun) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	if ctx == nil || arg == nil {
		return false
	}

	p, err := parseTradeParam(arg.CustomActionParam)
	if err != nil {
		focus(ctx, fmt.Sprintf("跑商：参数无效（%v），已中止", err), errorDisplay)
		logf("param error: %v", err)
		return false
	}

	tasker := ctx.GetTasker()
	if tasker == nil {
		focus(ctx, "跑商：拿不到 Tasker，已中止", errorDisplay)
		return false
	}
	ctrl := tasker.GetController()
	if ctrl == nil {
		focus(ctx, "跑商：拿不到 Controller（游戏未连接？），已中止", errorDisplay)
		return false
	}

	logf("run: dry_run=%v sell_mode=%s enable_sell=%v enable_buy=%v bargain=%v online_update=%v",
		p.DryRun, p.SellMode, p.EnableSell, p.EnableBuy, p.Bargain, p.OnlineUpdate)

	// 1. Load calendar + decide today's sell list.
	cal, day, err := loadCalendar(p)
	if err != nil {
		focus(ctx, fmt.Sprintf("跑商：价表加载失败（%v），已中止", err), errorDisplay)
		logf("calendar load failed: %v", err)
		return false
	}
	sell := cal.sellList(day)
	planStr := describePlan(cal, day, sell)
	logf("plan: %s", planStr)
	focus(ctx, "跑商计划："+planStr, logDisplay)

	if len(sell) == 0 && !p.EnableBuy {
		focus(ctx, "跑商：今日无峰值商品且未启用采购，已结束", logDisplay)
		return true
	}

	// 2. Navigate to the Q_sp6 merchant (pipeline-driven).
	if p.DryRun {
		logf("[dry-run] skip navigation")
	} else {
		if _, err := ctx.RunTask("Trade_EnterMerchant"); err != nil {
			focus(ctx, fmt.Sprintf("跑商：进入商人失败（%v），已中止", err), errorDisplay)
			logf("enter merchant failed: %v", err)
			return false
		}
	}

	// 3. Sell loop.
	sold := runSellLoop(ctx, ctrl, sell, p)

	// 4. Buy flow (bargain + buy all favorites).
	bought := runBuyFlow(ctx, ctrl, p)

	// 5. Return home.
	if !p.DryRun {
		if _, err := ctx.RunTask("Trade_ReturnHome"); err != nil {
			logf("return home failed: %v", err)
		}
	}

	summary := fmt.Sprintf("跑商完成：卖出 %d 件，采购 %v。%s", sold, bought, planStr)
	logf("summary: %s", summary)
	focus(ctx, summary, logDisplay)
	return true
}
