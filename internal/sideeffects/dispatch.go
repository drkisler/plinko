// plinko/internal/sideeffects/dispatch.go
/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package sideeffects

import (
	"context"

	"github.com/drkisler/plinko"
)

// AllowAllSideEffects 是一个便捷常量，表示注册为全局副作用（所有阶段都触发）。
const AllowAllSideEffects = plinko.AllowBeforeTransition | plinko.AllowAfterTransition | plinko.AllowBetweenStates

// SideEffectDefinition 保存副作用函数及其触发阶段的过滤器。
type SideEffectDefinition struct {
	SideEffect plinko.SideEffect
	Filter     plinko.SideEffectFilter
}

// getFilterDefinition 将 StateAction 映射为对应的 SideEffectFilter 位。
func getFilterDefinition(stateAction plinko.StateAction) plinko.SideEffectFilter {
	switch stateAction {
	case plinko.BeforeTransition:
		return plinko.AllowBeforeTransition
	case plinko.BetweenStates:
		return plinko.AllowBetweenStates
	case plinko.AfterTransition:
		return plinko.AllowAfterTransition
	}
	return 0
}

// TransitionDef 是 sideeffects 包内部使用的转换定义，实现了 TransitionInfo 和 ModifiableTransitionInfo 接口。
type TransitionDef struct {
	Source      plinko.State
	Destination plinko.State
	Trigger     plinko.Trigger
}

func (td TransitionDef) GetSource() plinko.State            { return td.Source }
func (td TransitionDef) GetDestination() plinko.State       { return td.Destination }
func (td *TransitionDef) SetDestination(state plinko.State) { td.Destination = state }
func (td TransitionDef) GetTrigger() plinko.Trigger         { return td.Trigger }

// Dispatch 遍历副作用列表，根据过滤器决定是否在给定阶段执行副作用函数。
// 返回实际执行的副作用数量。
func Dispatch(ctx context.Context, stateAction plinko.StateAction, sideEffects []SideEffectDefinition, payload plinko.Payload, transitionInfo plinko.TransitionInfo, elapsedMilliseconds int64) int {
	iCount := 0
	for _, sideEffectDefinition := range sideEffects {
		if sideEffectDefinition.Filter&getFilterDefinition(stateAction) > 0 {
			sideEffectDefinition.SideEffect(ctx, stateAction, payload, transitionInfo, elapsedMilliseconds)
			iCount++
		}
	}
	return iCount
}
