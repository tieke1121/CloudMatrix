package media

import "time"

// QualityProfile 表示视频流画质与资源控制等级
type QualityProfile string

const (
	// QualityFull 全帧率全高清模式，用于当前焦点交互
	QualityFull QualityProfile = "full"
	// QualityWallPreview 视频墙网格概览模式，中画质与适中帧率
	QualityWallPreview QualityProfile = "wall_preview"
	// QualityThrottled 后台遮挡节流模式，超低帧率以节约计算资源
	QualityThrottled QualityProfile = "throttled"
	// QualityPaused 视频流挂起/暂停模式，不进行任何采集与解码
	QualityPaused QualityProfile = "paused"
)

// StreamConfig 媒体流参数配置
type StreamConfig struct {
	InstanceID   string         `json:"instance_id"`
	TargetFPS    int            `json:"target_fps"`
	MaxBitrate   int            `json:"max_bitrate"`
	MaxWidth     int            `json:"max_width"`
	MaxHeight    int            `json:"max_height"`
	Profile      QualityProfile `json:"profile"`
	BufferSize   int            `json:"buffer_size"`
	DrainTimeout time.Duration  `json:"drain_timeout"`
}

// DefaultStreamConfig 返回对应画质等级的默认推荐配置
func DefaultStreamConfig(instanceID string, profile QualityProfile) StreamConfig {
	cfg := StreamConfig{
		InstanceID:   instanceID,
		Profile:      profile,
		BufferSize:   30,
		DrainTimeout: 500 * time.Millisecond,
	}

	switch profile {
	case QualityFull:
		cfg.TargetFPS = 30
		cfg.MaxBitrate = 4000000
		cfg.MaxWidth = 1920
		cfg.MaxHeight = 1080
	case QualityWallPreview:
		cfg.TargetFPS = 12
		cfg.MaxBitrate = 1200000
		cfg.MaxWidth = 960
		cfg.MaxHeight = 540
	case QualityThrottled:
		cfg.TargetFPS = 3
		cfg.MaxBitrate = 300000
		cfg.MaxWidth = 640
		cfg.MaxHeight = 360
	case QualityPaused:
		cfg.TargetFPS = 0
		cfg.MaxBitrate = 0
		cfg.MaxWidth = 0
		cfg.MaxHeight = 0
	default:
		cfg.Profile = QualityWallPreview
		cfg.TargetFPS = 12
		cfg.MaxBitrate = 1200000
		cfg.MaxWidth = 960
		cfg.MaxHeight = 540
	}

	return cfg
}
