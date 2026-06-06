// plinko/internal/composition/composition.go
/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package composition

import (
	"context"
	"runtime/debug"

	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/internal/sideeffects"
	"github.com/drkisler/plinko/plinkoerror"
)

// ChainedFunctionCall 表示一个带有可选守卫条件（Predicate）的操作调用。
// 如果 Predicate 不为 nil，则仅在它返回 nil 时才执行 Operation。
type ChainedFunctionCall struct {
	Predicate plinko.Predicate
	Operation plinko.Operation
	Config    plinko.OperationConfig
}

// ChainedErrorCall 表示一个错误处理操作。
type ChainedErrorCall struct {
	ErrorOperation plinko.ErrorOperation
	Config         plinko.OperationConfig
}

// CallbackDefinitions 聚合了一个状态的所有回调链（进入、退出、错误）。
type CallbackDefinitions struct {
	OnEntryFn []ChainedFunctionCall
	OnExitFn  []ChainedFunctionCall
	OnErrorFn []ChainedErrorCall

	// 这些字段似乎是历史遗留或用于其他用途，代码中未直接使用
	EntryFunctionChain []string
	ExitFunctionChain  []string
}

// AddError 添加一个错误处理回调。
func (cd *CallbackDefinitions) AddError(errorOperation plinko.ErrorOperation, cfg plinko.OperationConfig) *CallbackDefinitions {
	cd.OnErrorFn = append(cd.OnErrorFn, ChainedErrorCall{
		ErrorOperation: errorOperation,
		Config:         cfg,
	})
	return cd
}

// AddEntry 添加一个进入回调，并可选一个守卫 Predicate。
func (cd *CallbackDefinitions) AddEntry(predicate plinko.Predicate, operation plinko.Operation, cfg plinko.OperationConfig) *CallbackDefinitions {
	cd.OnEntryFn = append(cd.OnEntryFn, ChainedFunctionCall{
		Predicate: predicate,
		Operation: operation,
		Config:    cfg,
	})
	return cd
}

// AddExit 添加一个退出回调，并可选一个守卫 Predicate。
func (cd *CallbackDefinitions) AddExit(predicate plinko.Predicate, operation plinko.Operation, cfg plinko.OperationConfig) *CallbackDefinitions {
	cd.OnExitFn = append(cd.OnExitFn, ChainedFunctionCall{
		Predicate: predicate,
		Operation: operation,
		Config:    cfg,
	})
	return cd
}

// executeChain 依次执行回调链。若某回调 Predicate 返回错误，则跳过该回调。
// 若某个 Operation 返回错误，则停止并返回错误。
// 使用 defer/recover 捕获 panic 并转换为 PlinkoPanicError。
func executeChain(ctx context.Context, funcs []ChainedFunctionCall, p plinko.Payload, t plinko.TransitionInfo) (retPayload plinko.Payload, err error) {
	var stepName string
	step := 0
	defer func() {
		if err1 := recover(); err1 != nil {
			stack := string(debug.Stack())
			retPayload = p
			err = plinkoerror.CreatePlinkoPanicError(err1, t, step, stepName, stack)
		}
	}()

	if len(funcs) > 0 {
		for _, fn := range funcs {
			stepName = fn.Config.Name
			if fn.Predicate != nil {
				// 守卫条件：若返回非 nil，则跳过本回调，继续下一个
				if err = fn.Predicate(ctx, p, t); err != nil {
					continue
				}
			}
			var e error
			p, e = fn.Operation(ctx, p, t)
			step++
			if e != nil {
				return p, e
			}
		}
	}
	return p, nil
}

// executeErrorChain 依次执行错误处理回调链，每个回调都可修改载荷和转换定义。
// 如果某个错误处理回调返回错误，则立即返回。
func executeErrorChain(ctx context.Context, funcs []ChainedErrorCall, p plinko.Payload, t *sideeffects.TransitionDef, err error) (retPayload plinko.Payload, retTd *sideeffects.TransitionDef, retErr error) {
	var stepName string
	step := 0
	defer func() {
		if err1 := recover(); err1 != nil {
			stack := string(debug.Stack())
			retPayload = p
			retTd = t
			retErr = plinkoerror.CreatePlinkoPanicError(err1, t, step, stepName, stack)
		}
	}()

	if len(funcs) > 0 {
		for _, fn := range funcs {
			stepName = fn.Config.Name
			var e error
			p, e = fn.ErrorOperation(ctx, p, t, err)
			if e != nil {
				return p, t, e
			}
		}
	}
	return p, t, err
}

// ExecuteExitChain 执行退出回调链。
func (cd *CallbackDefinitions) ExecuteExitChain(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	return executeChain(ctx, cd.OnExitFn, p, t)
}

// ExecuteEntryChain 执行进入回调链。
func (cd *CallbackDefinitions) ExecuteEntryChain(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	return executeChain(ctx, cd.OnEntryFn, p, t)
}

// ExecuteErrorChain 执行错误回调链，并返回可能修改后的转换定义。
func (cd *CallbackDefinitions) ExecuteErrorChain(ctx context.Context, p plinko.Payload, t *sideeffects.TransitionDef, err error, elapsedMilliseconds int64) (plinko.Payload, *sideeffects.TransitionDef, error) {
	p, mt, err := executeErrorChain(ctx, cd.OnErrorFn, p, t, err)
	return p, mt, err
}
