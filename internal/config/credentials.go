package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type Credentials struct {
	Username string
	Password string
}

// Resolve uses the username flag, environment variables, then interactive input.
// Credentials are kept in memory and never written to disk.
func Resolve(username string, in io.Reader, out io.Writer) (Credentials, error) {
	c := Credentials{Username: username, Password: os.Getenv("CAMPUS_PASSWORD")}
	if c.Username == "" {
		c.Username = os.Getenv("CAMPUS_USERNAME")
	}
	reader := bufio.NewReader(in)
	for _, field := range []struct {
		key, prompt string
		value       *string
		secret      bool
	}{
		{"CAMPUS_USERNAME", "请输入校园网账号：", &c.Username, false},
		{"CAMPUS_PASSWORD", "请输入校园网密码：", &c.Password, true},
	} {
		if *field.value != "" {
			continue
		}
		fmt.Fprint(out, field.prompt)
		var value string
		var readErr error
		if file, ok := in.(*os.File); ok && field.secret && term.IsTerminal(int(file.Fd())) {
			var data []byte
			data, readErr = term.ReadPassword(int(file.Fd()))
			fmt.Fprintln(out)
			value = string(data)
		} else {
			value, readErr = reader.ReadString('\n')
		}
		if readErr != nil && !(errors.Is(readErr, io.EOF) && len(value) > 0) {
			return Credentials{}, fmt.Errorf("缺少 %s，请设置环境变量或在终端输入: %w", field.key, readErr)
		}
		value = strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r")
		if value == "" {
			return Credentials{}, fmt.Errorf("%s 不能为空", field.key)
		}
		*field.value = value
	}
	return c, nil
}
