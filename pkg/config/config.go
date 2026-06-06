// plinko/pkg/config/config.go

package config

import (
	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/internal/runtime"
)

// CreatePlinkoDefinition 创建一个新的 PlinkoDefinition 实例，用于构建状态机。
func CreatePlinkoDefinition() plinko.PlinkoDefinition {
	stateMap := make(map[plinko.State]*runtime.InternalStateDefinition)
	p := runtime.PlinkoDefinition{
		States: &stateMap,
	}
	p.Abs = runtime.AbstractSyntax{}
	return &p
}
