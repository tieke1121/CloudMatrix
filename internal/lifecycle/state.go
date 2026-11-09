package lifecycle

import (
	"cloudmatrix/internal/domain"
)

// validTransitions 状态机合法流转规则表
var validTransitions = map[domain.InstanceState][]domain.InstanceState{
	domain.StateDiscovered: {domain.StateStopped, domain.StateStarting, domain.StateUnknown},
	domain.StateStopped:    {domain.StateStarting, domain.StateUnknown},
	domain.StateStarting:   {domain.StateBooting, domain.StateStopping, domain.StateStopped, domain.StateError, domain.StateUnknown},
	domain.StateBooting:    {domain.StateReady, domain.StateStopping, domain.StateStopped, domain.StateError, domain.StateUnknown},
	domain.StateReady:      {domain.StateStopping, domain.StateStopped, domain.StateError, domain.StateUnknown},
	domain.StateStopping:   {domain.StateStopped, domain.StateError, domain.StateUnknown},
	// error 和 unknown 状态绝不自动回退到 stopped，仅允许人工触发显式 starting、stopping 或刷新
	domain.StateError:      {domain.StateStarting, domain.StateStopping, domain.StateUnknown},
	domain.StateUnknown:    {domain.StateStarting, domain.StateStopping, domain.StateReady, domain.StateStopped, domain.StateError},
}

// CanTransition 校验状态转换是否合法
func CanTransition(from domain.InstanceState, to domain.InstanceState) bool {
	if from == to {
		return true
	}

	targets, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, target := range targets {
		if target == to {
			return true
		}
	}
	return false
}
