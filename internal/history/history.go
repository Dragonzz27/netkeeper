package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

const TimeLayout = "2006-01-02 15:04:05"

// Entry retains the original Python JSONL schema.
type Entry struct {
	Timestamp  string   `json:"timestamp"`
	LastLogin  *string  `json:"last_login"`
	ElapsedMin *float64 `json:"elapsed_min"`
}

func NewEntry(now time.Time, last *time.Time) Entry {
	e := Entry{Timestamp: now.Format(TimeLayout)}
	if last != nil {
		formatted := last.Format(TimeLayout)
		elapsed := now.Sub(*last).Minutes()
		e.LastLogin, e.ElapsedMin = &formatted, &elapsed
	}
	return e
}

func ParseTime(value string) (time.Time, error) {
	if parsed, err := time.ParseInLocation(TimeLayout, value, time.Local); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

func Read(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries := []Entry{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("日志第 %d 行: %w", line, err)
		}
		now, err := ParseTime(e.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("日志第 %d 行时间无效: %w", line, err)
		}
		if e.ElapsedMin == nil && e.LastLogin != nil {
			previous, err := ParseTime(*e.LastLogin)
			if err != nil {
				return nil, fmt.Errorf("日志第 %d 行 last_login 无效: %w", line, err)
			}
			elapsed := now.Sub(previous).Minutes()
			e.ElapsedMin = &elapsed
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取日志: %w", err)
	}
	return entries, nil
}

func Append(path string, e Entry) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开登录日志: %w", err)
	}
	err = json.NewEncoder(f).Encode(e)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("写入登录日志: %w", err)
	}
	return closeErr
}

func Last(path string) (*time.Time, error) {
	entries, err := Read(path)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	last, err := ParseTime(entries[len(entries)-1].Timestamp)
	return &last, err
}

func Pretty(minutes *float64) string {
	if minutes == nil {
		return "-"
	}
	n := int(*minutes)
	days, hours, mins := n/(24*60), (n/60)%24, n%60
	result := ""
	if days > 0 {
		result += fmt.Sprintf("%d 天 ", days)
	}
	if hours > 0 {
		result += fmt.Sprintf("%d 小时 ", hours)
	}
	if mins > 0 || result == "" {
		result += fmt.Sprintf("%d 分钟", mins)
	}
	if len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}
	return result
}
