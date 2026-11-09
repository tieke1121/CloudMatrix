package stream

import (
	"errors"
	"fmt"
	"strings"
)

// ScrcpyOptions 投屏运行参数配置
type ScrcpyOptions struct {
	Serial      string `json:"serial"`
	WindowTitle string `json:"window_title"`
	NoAudio     bool   `json:"no_audio"`
	StayAwake   bool   `json:"stay_awake"`
	MaxFPS      int    `json:"max_fps"`
	MaxSize     int    `json:"max_size"`
}

// BuildScrcpyArgs 构造经过严格参数切片化隔离的 scrcpy 命令行参数
func BuildScrcpyArgs(opts ScrcpyOptions) ([]string, error) {
	if strings.TrimSpace(opts.Serial) == "" {
		return nil, errors.New("scrcpy 必须指定设备 serial")
	}

	args := []string{"--serial", opts.Serial}

	// 窗口标题
	title := strings.TrimSpace(opts.WindowTitle)
	if title == "" {
		title = fmt.Sprintf("CloudMatrix - %s", opts.Serial)
	}
	args = append(args, "--window-title", title)

	// 关闭非必要音频以显著降低宿主 CPU 与延迟开销
	if opts.NoAudio {
		args = append(args, "--no-audio")
	}

	// 保持屏幕常亮
	if opts.StayAwake {
		args = append(args, "--stay-awake")
	}

	// 限制最高帧率
	if opts.MaxFPS > 0 {
		args = append(args, fmt.Sprintf("--max-fps=%d", opts.MaxFPS))
	}

	// 限制最大长宽尺寸
	if opts.MaxSize > 0 {
		args = append(args, fmt.Sprintf("--max-size=%d", opts.MaxSize))
	}

	return args, nil
}
