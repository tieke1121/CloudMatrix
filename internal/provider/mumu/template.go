package mumu

import (
	"errors"
	"strings"
)

// FormatCloneCommand 构造 MuMu 实例克隆参数切片
func FormatCloneCommand(sourceIndex string, targetName string) ([]string, error) {
	if strings.TrimSpace(sourceIndex) == "" {
		return nil, errors.New("源实例索引不能为空")
	}
	args := []string{"clone", "--index", sourceIndex}
	if strings.TrimSpace(targetName) != "" {
		args = append(args, "--name", targetName)
	}
	return args, nil
}

// FormatResetCommand 构造 MuMu 实例重置恢复参数切片
func FormatResetCommand(index string) ([]string, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("实例索引不能为空")
	}
	return []string{"reset", "--index", index}, nil
}

// FormatDeleteCommand 构造 MuMu 实例删除参数切片
func FormatDeleteCommand(index string) ([]string, error) {
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("实例索引不能为空")
	}
	return []string{"delete", "--index", index}, nil
}
