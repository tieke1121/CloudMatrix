package mumu

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"cloudmatrix/internal/domain"
)

// RawInstance CLI 返回的原始实例信息结构
type RawInstance struct {
	Index   interface{} `json:"index"`
	Name    string      `json:"name"`
	Running interface{} `json:"running"`
	AdbPort interface{} `json:"adb_port"`
	Serial  string      `json:"serial"`
}

// ParseCLIOutput 解析 MuMu CLI 输出内容，并转换为内部领域模型
func ParseCLIOutput(output []byte) ([]*domain.Instance, error) {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "[]" {
		return []*domain.Instance{}, nil
	}

	var rawList []RawInstance
	if err := json.Unmarshal([]byte(trimmed), &rawList); err != nil {
		return nil, fmt.Errorf("解析 MuMu CLI 输出 JSON 失败: %w", err)
	}

	seenIDs := make(map[string]bool)
	var instances []*domain.Instance

	for _, item := range rawList {
		// 解析实例索引/标识
		var instID string
		switch v := item.Index.(type) {
		case float64:
			instID = strconv.Itoa(int(v))
		case string:
			instID = strings.TrimSpace(v)
		default:
			continue
		}

		if instID == "" {
			continue
		}

		// 过滤重复 ID
		if seenIDs[instID] {
			continue
		}
		seenIDs[instID] = true

		observedState := parseRunningState(item.Running)
		desiredState := observedState
		if observedState == domain.StateBooting {
			desiredState = domain.StateReady
		}

		// 解析 ADB serial，不得假定 5555+2N
		adbSerial := strings.TrimSpace(item.Serial)
		if adbSerial == "" && item.AdbPort != nil {
			var port int
			switch p := item.AdbPort.(type) {
			case float64:
				port = int(p)
			case string:
				port, _ = strconv.Atoi(strings.TrimSpace(p))
			}
			if port > 0 && port <= 65535 {
				adbSerial = fmt.Sprintf("127.0.0.1:%d", port)
			}
		}

		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("MuMu-%s", instID)
		}

		instances = append(instances, &domain.Instance{
			ID:                 fmt.Sprintf("mumu-local-%s", instID),
			HostID:             "local",
			ProviderType:       "mumu",
			ProviderInstanceID: instID,
			Name:               name,
			DesiredState:       desiredState,
			ObservedState:      observedState,
			AdbSerial:          adbSerial,
		})
	}

	runningSerials := make(map[string]string)
	for _, instance := range instances {
		if instance.ObservedState != domain.StateBooting || instance.AdbSerial == "" {
			continue
		}
		if other, exists := runningSerials[instance.AdbSerial]; exists {
			return nil, fmt.Errorf("两个运行中的实例报告了相同的 ADB 绑定（%s、%s）", other, instance.ProviderInstanceID)
		}
		runningSerials[instance.AdbSerial] = instance.ProviderInstanceID
	}
	return instances, nil
}

func parseRunningState(value any) domain.InstanceState {
	switch value := value.(type) {
	case bool:
		if value {
			return domain.StateBooting
		}
		return domain.StateStopped
	case float64:
		switch value {
		case 1:
			return domain.StateBooting
		case 0:
			return domain.StateStopped
		}
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1":
			return domain.StateBooting
		case "false", "0":
			return domain.StateStopped
		}
	}
	return domain.StateUnknown
}

// FormatStartCommand 构造启动命令参数切片
func FormatStartCommand(index string) ([]string, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("实例标识不能为空")
	}
	return []string{"launch", "--index", index}, nil
}

// FormatStopCommand 构造停止命令参数切片
func FormatStopCommand(index string) ([]string, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("实例标识不能为空")
	}
	return []string{"shutdown", "--index", index}, nil
}
