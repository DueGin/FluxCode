package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type usageWindow struct {
	name  string
	used  float64
	reset *time.Time
}

func usageWindowsFromUsageInfo(usage *UsageInfo) []usageWindow {
	if usage == nil {
		return nil
	}
	windows := make([]usageWindow, 0, 3)
	if usage.FiveHour != nil {
		windows = append(windows, usageWindow{
			name:  "5h",
			used:  usage.FiveHour.Utilization,
			reset: usage.FiveHour.ResetsAt,
		})
	}
	if usage.SevenDay != nil {
		windows = append(windows, usageWindow{
			name:  "7d",
			used:  usage.SevenDay.Utilization,
			reset: usage.SevenDay.ResetsAt,
		})
	}
	if usage.SevenDaySonnet != nil {
		windows = append(windows, usageWindow{
			name:  "7d_sonnet",
			used:  usage.SevenDaySonnet.Utilization,
			reset: usage.SevenDaySonnet.ResetsAt,
		})
	}
	return windows
}

func exceededWindows(windows []usageWindow, threshold float64) []usageWindow {
	if len(windows) == 0 {
		return nil
	}
	if threshold <= 0 {
		threshold = defaultUsageWindowDisablePercent
	}
	var exceeded []usageWindow
	for _, w := range windows {
		if w.used >= threshold {
			exceeded = append(exceeded, w)
		}
	}
	return exceeded
}

func buildUsageExceededReason(platform string, exceeded []usageWindow) string {
	platformLabel := "账号"
	switch platform {
	case PlatformAnthropic:
		platformLabel = "Anthropic"
	case PlatformOpenAI:
		platformLabel = "OpenAI"
	case PlatformGemini:
		platformLabel = "Gemini"
	case PlatformAntigravity:
		platformLabel = "Antigravity"
	}

	var b strings.Builder
	b.WriteString(platformLabel)
	b.WriteString(" 额度已超限：")
	for i, w := range exceeded {
		if i > 0 {
			b.WriteString("；")
		}
		b.WriteString(w.name)
		b.WriteString(" 已用 ")
		b.WriteString(strconv.FormatFloat(w.used, 'f', 1, 64))
		b.WriteString("%")
		if w.reset != nil {
			b.WriteString("，预计 ")
			b.WriteString(w.reset.Format(time.RFC3339))
			b.WriteString(" 恢复")
		} else {
			b.WriteString("（重置时间未知）")
		}
	}
	return strings.TrimSpace(b.String())
}

func codexUsageWindows(account *Account, now time.Time, threshold float64) ([]usageWindow, []usageWindow) {
	if account == nil || account.Extra == nil {
		return nil, nil
	}
	if threshold <= 0 {
		threshold = defaultUsageWindowDisablePercent
	}
	baseTime := now
	if t, ok := getExtraTime(account.Extra, "codex_usage_updated_at"); ok {
		baseTime = t
	}

	windows := make([]usageWindow, 0, 2)
	exceeded := make([]usageWindow, 0, 2)

	if used, ok := getExtraFloat(account.Extra, "codex_5h_used_percent"); ok {
		if resetSeconds, okReset := getExtraInt(account.Extra, "codex_5h_reset_after_seconds"); okReset && resetSeconds > 0 {
			resetAt := baseTime.Add(time.Duration(resetSeconds) * time.Second)
			w := usageWindow{name: "5h", used: used, reset: &resetAt}
			windows = append(windows, w)
			if used >= threshold && resetAt.After(now) {
				exceeded = append(exceeded, w)
			}
		}
	}

	if used, ok := getExtraFloat(account.Extra, "codex_7d_used_percent"); ok {
		if resetSeconds, okReset := getExtraInt(account.Extra, "codex_7d_reset_after_seconds"); okReset && resetSeconds > 0 {
			resetAt := baseTime.Add(time.Duration(resetSeconds) * time.Second)
			w := usageWindow{name: "7d", used: used, reset: &resetAt}
			windows = append(windows, w)
			if used >= threshold && resetAt.After(now) {
				exceeded = append(exceeded, w)
			}
		}
	}

	return windows, exceeded
}

func getExtraFloat(extra map[string]any, key string) (float64, bool) {
	if extra == nil {
		return 0, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func getExtraInt(extra map[string]any, key string) (int, bool) {
	if extra == nil {
		return 0, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func getExtraTime(extra map[string]any, key string) (time.Time, bool) {
	if extra == nil {
		return time.Time{}, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return time.Time{}, false
	}
	switch val := v.(type) {
	case time.Time:
		return val, true
	case string:
		s := strings.TrimSpace(val)
		if s == "" {
			return time.Time{}, false
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
