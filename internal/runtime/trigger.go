// plinko/internal/runtime/trigger.go

/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/internal/sideeffects"
	"github.com/drkisler/plinko/plinkoerror"
)

// EnumerateActiveTriggers 返回当前状态允许的所有触发器列表。
func (psm plinkoStateMachine) EnumerateActiveTriggers(payload plinko.Payload) ([]plinko.Trigger, error) {
	state := payload.GetState()
	sd2 := (*psm.pd.States)[state]
	if sd2 == nil {
		return nil, plinkoerror.CreatePlinkoStateError(state, fmt.Sprintf("State %s not found in state machine definition", state))
	}
	keys := make([]plinko.Trigger, 0, len(sd2.Triggers))
	for k := range sd2.Triggers {
		keys = append(keys, k)
	}
	return keys, nil
}

// CanFire 检查是否满足触发条件（包括守卫条件）。
func (psm plinkoStateMachine) CanFire(ctx context.Context, payload plinko.Payload, trigger plinko.Trigger) error {
	state := payload.GetState()
	sd2 := (*psm.pd.States)[state]
	if sd2 == nil {
		return plinkoerror.CreatePlinkoStateError(state, fmt.Sprintf("State '%s' not defined", state))
	}
	triggerData := sd2.Triggers[trigger]
	if triggerData == nil {
		return plinkoerror.CreatePlinkoTriggerError(trigger, fmt.Sprintf("Triggers '%s' not defined for state '%s'", trigger, state))
	}
	if triggerData.Predicate != nil {
		return triggerData.Predicate(ctx, payload, sideeffects.TransitionDef{
			Destination: triggerData.DestinationState,
			Source:      state,
			Trigger:     triggerData.Name,
		})
	}
	return nil
}

// Fire 执行一次状态转换。
// 流程：
// 1. 校验当前状态和触发器。
// 2. 执行守卫条件（如有）。
// 3. 若为动态解析，则计算目标状态。
// 4. 调用 BeforeTransition 副作用。
// 5. 执行源状态退出回调链，若出错则执行错误回调。
// 6. 调用 BetweenStates 副作用。
// 7. 执行目标状态进入回调链，若出错则执行错误回调。
// 8. 调用 AfterTransition 副作用。
func (psm plinkoStateMachine) Fire(ctx context.Context, payload plinko.Payload, trigger plinko.Trigger) (plinko.Payload, error) {
	start := time.Now()
	state := payload.GetState()
	sd2 := (*psm.pd.States)[state]
	if sd2 == nil {
		return payload, plinkoerror.CreatePlinkoStateError(state, fmt.Sprintf("State not found in definition of states: %s", state))
	}

	triggerData := sd2.Triggers[trigger]
	if triggerData == nil {
		return payload, plinkoerror.CreatePlinkoTriggerError(trigger, fmt.Sprintf("Trigger '%s' not found in definition for state: %s", trigger, state))
	}

	td := &sideeffects.TransitionDef{
		Source:      state,
		Destination: triggerData.DestinationState,
		Trigger:     trigger,
	}

	// 守卫条件检查
	if triggerData.Predicate != nil {
		if err := triggerData.Predicate(ctx, payload, td); err != nil {
			return payload, plinkoerror.CreatePlinkoTriggerError(trigger, fmt.Sprintf("Conditional Trigger '%s' conditions not met for state: %s", trigger, state))
		}
	}

	// 动态解析目标状态
	if triggerData.DynamicResolver != nil {
		dynamicState, err := triggerData.DynamicResolver(ctx, payload, td)
		if err != nil {
			return payload, err
		}
		td.SetDestination(dynamicState)
	}

	// 获取目标状态定义
	destinationState := (*psm.pd.States)[td.GetDestination()]
	if destinationState == nil {
		return payload, plinkoerror.CreatePlinkoStateError(td.GetDestination(), fmt.Sprintf("Destination state '%s' not found", td.GetDestination()))
	}

	// 触发 BeforeTransition 副作用
	sideeffects.Dispatch(ctx, plinko.BeforeTransition, psm.pd.SideEffects, payload, td, time.Since(start).Milliseconds())

	// 执行源状态的退出回调链
	payload, err := sd2.Callbacks.ExecuteExitChain(ctx, payload, td)
	if err != nil {
		// 如果退出回调出错，执行源状态的错误处理链
		payload, td, errSub := sd2.Callbacks.ExecuteErrorChain(ctx, payload, td, err, time.Since(start).Milliseconds())
		if errSub != nil {
			err = errSub
		}
		sideeffects.Dispatch(ctx, plinko.BetweenStates, psm.pd.SideEffects, payload, td, time.Since(start).Milliseconds())
		return payload, err
	}

	// BetweenStates 副作用
	sideeffects.Dispatch(ctx, plinko.BetweenStates, psm.pd.SideEffects, payload, td, time.Since(start).Milliseconds())

	// 执行目标状态的进入回调链
	payload, err = destinationState.Callbacks.ExecuteEntryChain(ctx, payload, td)
	if err != nil {
		// 如果进入回调出错，执行目标状态的错误处理链
		var errSub error
		payload, mtd, errSub := destinationState.Callbacks.ExecuteErrorChain(ctx, payload, td, err, time.Since(start).Milliseconds())
		td = mtd
		if errSub != nil {
			err = errSub
		}
		return payload, err
	}

	// AfterTransition 副作用
	sideeffects.Dispatch(ctx, plinko.AfterTransition, psm.pd.SideEffects, payload, td, time.Since(start).Milliseconds())

	return payload, nil
}
