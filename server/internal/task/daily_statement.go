// Package task 进程内定时任务(当前仅财务日结)。
// 不引入调度框架:单任务 + 固定时刻,goroutine + Timer 足够,
// 漏跑由「启动补跑 + 手动触发接口」双兜底(聚合幂等,重复执行无副作用)。
package task

import (
	"context"
	"time"

	"go.uber.org/zap"

	"fawn-mall/pkg/logger"
)

// StatementRunner 日结执行方(*service.FinanceService 实现)。
type StatementRunner interface {
	RunDailyStatement(ctx context.Context, date string) error
}

// cstLoc 业务时区:与 service 层财务口径一致,固定北京时间。
var cstLoc = time.FixedZone("Asia/Shanghai", 8*3600)

// StartDailyStatement 启动日结定时任务:
// 立即补跑一次昨日(防进程重启/停机漏跑),此后每天 02:00 聚合昨日流水。
// ctx 取消时退出。
func StartDailyStatement(ctx context.Context, r StatementRunner) {
	go func() {
		yesterday := time.Now().In(cstLoc).AddDate(0, 0, -1).Format("2006-01-02")
		if err := r.RunDailyStatement(ctx, yesterday); err != nil {
			logger.Warn(ctx, "启动补跑昨日日结失败(不影响服务)", zap.String("stat_date", yesterday), zap.Error(err))
		} else {
			logger.Info(ctx, "启动补跑昨日日结完成", zap.String("stat_date", yesterday))
		}
		for {
			timer := time.NewTimer(time.Until(nextTwoAM(time.Now())))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				date := time.Now().In(cstLoc).AddDate(0, 0, -1).Format("2006-01-02")
				if err := r.RunDailyStatement(ctx, date); err != nil {
					logger.Error(ctx, "定时日结失败", zap.String("stat_date", date), zap.Error(err))
				} else {
					logger.Info(ctx, "定时日结完成", zap.String("stat_date", date))
				}
			}
		}
	}()
}

// nextTwoAM 下一个北京时间 02:00(绝对时刻,跨时区部署同样正确)。
func nextTwoAM(now time.Time) time.Time {
	n := now.In(cstLoc)
	next := time.Date(n.Year(), n.Month(), n.Day(), 2, 0, 0, 0, cstLoc)
	if !n.Before(next) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
