package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Dragonzz27/netkeeper/internal/config"
	"github.com/Dragonzz27/netkeeper/internal/history"
)

type Portal interface {
	Check(context.Context) (bool, error)
	Login(context.Context, string, string) error
}

type Runner struct {
	Portal      Portal
	Credentials config.Credentials
	LogFile     string
	Out         io.Writer
	LastLogin   *time.Time
}

func (r *Runner) Login(ctx context.Context) error {
	if err := r.Portal.Login(ctx, r.Credentials.Username, r.Credentials.Password); err != nil {
		return err
	}
	now := time.Now()
	entry := history.NewEntry(now, r.LastLogin)
	if err := history.Append(r.LogFile, entry); err != nil {
		return err
	}
	r.LastLogin = &now
	fmt.Fprintf(r.Out, "[%s] 登录成功，距上次登录：%s\n", entry.Timestamp, history.Pretty(entry.ElapsedMin))
	return nil
}

func (r *Runner) Once(ctx context.Context) error {
	online, err := r.Portal.Check(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		fmt.Fprintln(r.Out, "网络检测异常，尝试登录：", err)
	}
	if online {
		fmt.Fprintf(r.Out, "[%s] 网络已登录，无需操作\n", time.Now().Format(history.TimeLayout))
		return nil
	}
	fmt.Fprintln(r.Out, "未登录，开始自动登录...")
	return r.Login(ctx)
}

func (r *Runner) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("interval 必须大于 0")
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.Once(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintln(r.Out, "自动登录失败，下次检测时重试：", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
