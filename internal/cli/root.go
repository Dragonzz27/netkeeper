package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/Dragonzz27/netkeeper/internal/config"
	"github.com/Dragonzz27/netkeeper/internal/dashboard"
	"github.com/Dragonzz27/netkeeper/internal/history"
	"github.com/Dragonzz27/netkeeper/internal/portal"
	"github.com/Dragonzz27/netkeeper/internal/runner"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var opts portal.Options
	var logFile, username string
	var interval time.Duration
	root := &cobra.Command{
		Use: "netkeeper", Short: "Linux 服务器校园网自动登录工具（华东理工大学）",
		SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	flags := root.PersistentFlags()
	flags.StringVar(&logFile, "log-file", "login_log.jsonl", "登录日志文件")
	flags.StringVarP(&username, "username", "u", "", "校园网账号（优先于环境变量）")
	flags.StringVar(&opts.Mode, "mode", "campus", "登录页面类型：campus 或 server")
	flags.StringVar(&opts.CheckURL, "check-url", "http://www.baidu.com", "用于验证联网状态的地址")
	flags.StringVar(&opts.PortalURL, "portal-url", "http://172.20.3.90:804/srun_portal_pc.php", "未发现重定向时使用的认证地址")
	flags.StringVar(&opts.ChromePath, "chrome-path", "", "Chrome/Chromium 可执行文件路径（默认自动查找）")
	flags.BoolVar(&opts.Headless, "headless", true, "使用无头浏览器")
	flags.BoolVar(&opts.NoSandbox, "no-sandbox", false, "禁用 Chrome 沙箱（仅在运行环境需要时设置）")
	flags.StringVar(&opts.Domain, "domain", "", "上网类型，如 @free（默认使用页面选中项）")
	flags.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "每次检测或登录的超时时间")
	flags.DurationVar(&interval, "interval", time.Minute, "自动检测间隔")
	newBrowser := func(cmd *cobra.Command) (*portal.Browser, error) {
		opts.DomainSet = cmd.Flags().Changed("domain")
		return portal.New(opts)
	}
	newRunner := func(cmd *cobra.Command) (*runner.Runner, error) {
		browser, err := newBrowser(cmd)
		if err != nil {
			return nil, err
		}
		last, err := history.Last(logFile)
		if err != nil {
			return nil, err
		}
		credentials, err := config.Resolve(username, cmd.InOrStdin(), cmd.ErrOrStderr())
		if err != nil {
			return nil, err
		}
		return &runner.Runner{Portal: browser, Credentials: credentials, LogFile: logFile, Out: cmd.OutOrStdout(), LastLogin: last}, nil
	}
	run := func(cmd *cobra.Command, _ []string) error {
		if interval <= 0 {
			return fmt.Errorf("interval 必须大于 0")
		}
		r, err := newRunner(cmd)
		if err != nil {
			return err
		}
		return r.Run(cmd.Context(), interval)
	}
	root.RunE = run
	root.AddCommand(&cobra.Command{Use: "run", Short: "持续检测网络并在掉线后自动登录（默认命令）", Args: cobra.NoArgs, RunE: run})
	root.AddCommand(&cobra.Command{Use: "login", Short: "执行一次登录，验证联网成功后写入日志", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := newRunner(cmd)
		if err != nil {
			return err
		}
		return r.Login(cmd.Context())
	}})
	root.AddCommand(&cobra.Command{Use: "status", Short: "检测一次联网状态（离线或检测失败时返回非零退出码）", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		browser, err := newBrowser(cmd)
		if err != nil {
			return err
		}
		online, err := browser.Check(cmd.Context())
		if err != nil {
			return err
		}
		if !online {
			return fmt.Errorf("网络未登录")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "网络已登录")
		return nil
	}})
	root.AddCommand(&cobra.Command{Use: "logs", Short: "查看登录时间和间隔", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		entries, err := history.Read(logFile)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "暂无登录日志")
			return nil
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "登录时间\t上次登录\t间隔")
		for _, e := range entries {
			last := "-"
			if e.LastLogin != nil {
				last = *e.LastLogin
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.Timestamp, last, history.Pretty(e.ElapsedMin))
		}
		return w.Flush()
	}})
	var addr string
	serve := &cobra.Command{Use: "serve", Short: "启动本地登录间隔图表与数据表", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return dashboard.Serve(cmd.Context(), addr, logFile, cmd.OutOrStdout())
	}}
	serve.Flags().StringVar(&addr, "addr", "127.0.0.1:8501", "日志页面监听地址")
	root.AddCommand(serve)
	return root
}
