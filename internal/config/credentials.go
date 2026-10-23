package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/term"
)

type Credentials struct {
	Username string
	Password string
}

// Resolve gives process environment variables precedence over the existing .env.
// Only interactively entered values are persisted; other keys are preserved.
func Resolve(path, username string, in io.Reader, out io.Writer) (Credentials, error) {
	values, err := godotenv.Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Credentials{}, fmt.Errorf("读取配置: %w", err)
	}
	if values == nil {
		values = make(map[string]string)
	}
	lookup := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return values[key]
	}
	c := Credentials{Username: username, Password: lookup("CAMPUS_PASSWORD")}
	if c.Username == "" {
		c.Username = lookup("CAMPUS_USERNAME")
	}
	reader := bufio.NewReader(in)
	changed := false
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
			return Credentials{}, fmt.Errorf("缺少 %s，请配置环境变量或 %s: %w", field.key, path, readErr)
		}
		value = strings.TrimSuffix(strings.TrimSuffix(value, "\n"), "\r")
		if value == "" {
			return Credentials{}, fmt.Errorf("%s 不能为空", field.key)
		}
		*field.value = value
		values[field.key] = value
		changed = true
	}
	if changed {
		if err := save(path, values); err != nil {
			return Credentials{}, err
		}
	}
	return c, nil
}

func save(path string, values map[string]string) error {
	data, err := godotenv.Marshal(values)
	if err != nil {
		return fmt.Errorf("编码配置: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ecust-env-*")
	if err != nil {
		return fmt.Errorf("创建配置: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err = io.WriteString(f, data+"\n"); err != nil {
		f.Close()
		return fmt.Errorf("写入配置: %w", err)
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("保存配置: %w", err)
	}
	return nil
}
