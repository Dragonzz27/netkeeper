package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type Options struct {
	Mode       string
	CheckURL   string
	PortalURL  string
	ChromePath string
	Headless   bool
	NoSandbox  bool
	Timeout    time.Duration
	Domain     string
	DomainSet  bool
}

type Browser struct{ opts Options }

func New(opts Options) (*Browser, error) {
	if opts.Mode != "campus" && opts.Mode != "server" {
		return nil, fmt.Errorf("mode 必须是 campus 或 server")
	}
	if opts.Timeout <= 0 {
		return nil, fmt.Errorf("timeout 必须大于 0")
	}
	for _, address := range []string{opts.CheckURL, opts.PortalURL} {
		u, err := url.Parse(address)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return nil, fmt.Errorf("无效的 HTTP(S) 地址: %q", address)
		}
	}
	candidates := []string{opts.ChromePath}
	if opts.ChromePath == "" {
		candidates = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "msedge",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium"}
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"), filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			opts.ChromePath = path
			return &Browser{opts: opts}, nil
		}
	}
	return nil, fmt.Errorf("找不到 Chrome/Chromium，请安装浏览器或使用 --chrome-path 指定可执行文件")
}

func (b *Browser) session(ctx context.Context) (context.Context, func()) {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(b.opts.ChromePath), chromedp.Flag("headless", b.opts.Headless))
	if b.opts.NoSandbox {
		opts = append(opts, chromedp.NoSandbox)
	}
	alloc, stopAlloc := chromedp.NewExecAllocator(ctx, opts...)
	browser, stopBrowser := chromedp.NewContext(alloc)
	limited, stopTimeout := context.WithTimeout(browser, b.opts.Timeout)
	return limited, func() { stopTimeout(); stopBrowser(); stopAlloc() }
}

// Check uses a browser so both HTTP redirects and JavaScript portal redirects work.
func (b *Browser) Check(ctx context.Context) (bool, error) {
	browser, stop := b.session(ctx)
	defer stop()
	return b.check(browser)
}

func (b *Browser) check(ctx context.Context) (bool, error) {
	var location string
	var hasLoginForm bool
	response, err := chromedp.RunResponse(ctx,
		chromedp.Navigate(b.opts.CheckURL),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&location),
		chromedp.Evaluate(`!!(document.querySelector('#username') && document.querySelector('#password'))`, &hasLoginForm),
	)
	if err != nil {
		return false, fmt.Errorf("检测网络: %w", err)
	}
	if response == nil || response.Status < 200 || response.Status >= 400 {
		return false, fmt.Errorf("检测地址未返回成功的 HTTP 响应")
	}
	return onlineURL(b.opts.CheckURL, location) && !hasLoginForm, nil
}

func onlineURL(checkURL, location string) bool {
	expected, err := url.Parse(checkURL)
	if err != nil {
		return false
	}
	actual, err := url.Parse(location)
	if err != nil || actual.Hostname() == "" || (actual.Scheme != "http" && actual.Scheme != "https") {
		return false
	}
	path := strings.ToLower(actual.Path)
	return strings.EqualFold(expected.Hostname(), actual.Hostname()) &&
		!strings.Contains(path, "srun_portal") && !strings.Contains(path, "login")
}

func (b *Browser) Login(ctx context.Context, username, password string) error {
	browser, stop := b.session(ctx)
	defer stop()
	if err := chromedp.Run(browser); err != nil {
		return fmt.Errorf("启动浏览器: %w", err)
	}
	var location string
	// Limit the initial external navigation so an unreachable site leaves time
	// to try the configured intranet portal within the overall login deadline.
	probeTimeout := min(b.opts.Timeout/3, 8*time.Second)
	probe, cancelProbe := context.WithTimeout(browser, probeTimeout)
	err := chromedp.Run(probe, chromedp.Navigate(b.opts.CheckURL), chromedp.Sleep(2*time.Second), chromedp.Location(&location))
	cancelProbe()
	if err != nil {
		// An unavailable external site must not prevent reaching the configured portal.
		if browser.Err() != nil {
			return fmt.Errorf("打开登录页面: %w", err)
		}
		location = b.opts.PortalURL
	}
	if onlineURL(b.opts.CheckURL, location) || location == "" {
		location = b.opts.PortalURL
	}
	actions := []chromedp.Action{
		chromedp.Navigate(location),
		chromedp.WaitVisible("#username", chromedp.ByQuery),
		chromedp.WaitVisible("#password", chromedp.ByQuery),
		chromedp.SetValue("#username", username, chromedp.ByQuery),
		chromedp.SetValue("#password", password, chromedp.ByQuery),
	}
	if b.opts.Mode == "server" || b.opts.DomainSet {
		domain, _ := json.Marshal(b.opts.Domain)
		script := `(() => {
			const target = document.querySelector('#domain');
			if (target) target.value = ` + string(domain) + `;
		})()`
		if !b.opts.DomainSet {
			script = `(() => {
				const target = document.querySelector('#domain');
				const selected = document.querySelector("input[name='net-type']:checked");
				if (target && selected) target.value = selected.value;
			})()`
		}
		actions = append(actions, chromedp.Evaluate(script, nil))
	}
	selector := "input[type='submit']"
	if b.opts.Mode == "server" {
		selector = "#login"
	}
	actions = append(actions, chromedp.WaitEnabled(selector, chromedp.ByQuery), chromedp.Click(selector, chromedp.ByQuery), chromedp.Sleep(3*time.Second))
	if err := chromedp.Run(browser, actions...); err != nil {
		return fmt.Errorf("提交登录表单: %w", err)
	}
	// A click alone is not success: verify that the external check URL is reachable.
	for {
		online, err := b.check(browser)
		if online {
			return nil
		}
		if browser.Err() != nil {
			return fmt.Errorf("登录后仍无法访问检测地址（请检查凭据、登录模式或检测地址）: %w", browser.Err())
		}
		if err != nil {
			return err
		}
		select {
		case <-browser.Done():
			return fmt.Errorf("登录验证超时: %w", browser.Err())
		case <-time.After(time.Second):
		}
	}
}
