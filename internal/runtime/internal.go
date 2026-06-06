// plinko/internal/runtime/internal.go

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
	"reflect"
	"runtime"
	"strings"

	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/internal/composition"
	"github.com/drkisler/plinko/internal/sideeffects"
)

// plinkoStateMachine 是可执行状态机，持有编译后的定义。
type plinkoStateMachine struct {
	pd PlinkoDefinition
}

// InternalStateDefinition 是内部的状态定义，包含触发器表、回调、元信息。
type InternalStateDefinition struct {
	State     plinko.State
	Triggers  map[plinko.Trigger]*TriggerDefinition
	info      plinko.StateConfig
	Callbacks *composition.CallbackDefinitions
	Abs       *AbstractSyntax // 指向顶层抽象语法树，用于注册触发器
}

// OnEntry 添加进入回调。如果未提供配置选项，自动生成默认名称（函数名+位置）。
func (sd InternalStateDefinition) OnEntry(entryFn plinko.Operation, opts ...plinko.OperationOption) plinko.StateDefinition {
	if opts == nil {
		opts = append(opts, func(c *plinko.OperationConfig) {
			c.Name = getCallerHelper(entryFn) // 获取调用位置信息作为名称
		})
	}
	sd.Callbacks.AddEntry(nil, entryFn, newOperationConfig(entryFn, opts...))
	return sd
}

// getCallerHelper 通过调用栈获取指定函数在源码中的位置（文件名:行号）。
func getCallerHelper(f interface{}) string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc) // 跳过 getCallerHelper 和 OnEntry 等
	frames := runtime.CallersFrames(pc[:n])
	fnName := nameOf(f)
	return checkCallstack(frames, fnName)
}

// checkCallstack 在调用栈中查找指定函数名，返回其文件名:行号。
func checkCallstack(frames *runtime.Frames, functionName string) string {
	if frames == nil {
		return functionName
	}
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		list := strings.Split(frame.Function, ".")
		if len(list) > 0 && list[len(list)-1] == functionName {
			return fmt.Sprintf("%s:%d", cleanFileName(frame.File), frame.Line)
		}
	}
	return functionName
}

// cleanFileName 截取文件路径最后三段，避免暴露全路径。
func cleanFileName(fileName string) string {
	paths := strings.Split(fileName, "/")
	if paths == nil || len(paths) < 3 {
		return fileName
	}
	return strings.Join(paths[len(paths)-3:], "/")
}

// nameOf 返回函数的简短名称，不包含包路径。
func nameOf(f interface{}) string {
	v := reflect.ValueOf(f)
	if v.Kind() == reflect.Func {
		if rf := runtime.FuncForPC(v.Pointer()); rf != nil {
			name := rf.Name()
			names := strings.Split(name, ".")
			if len(names) > 1 {
				name = names[len(names)-1]
				if name == "func1" { // 匿名函数
					name = names[len(names)-2]
				}
			}
			return name
		}
	}
	return "anonymous_function:" + v.String()
}

// OnError 添加错误处理回调。
func (sd InternalStateDefinition) OnError(errorFn plinko.ErrorOperation, opts ...plinko.OperationOption) plinko.StateDefinition {
	sd.Callbacks.AddError(errorFn, newOperationConfig(errorFn, opts...))
	return sd
}

// OnExit 添加退出回调。
func (sd InternalStateDefinition) OnExit(exitFn plinko.Operation, opts ...plinko.OperationOption) plinko.StateDefinition {
	sd.Callbacks.AddExit(nil, exitFn, newOperationConfig(exitFn, opts...))
	return sd
}

// OnTriggerEntry 添加针对特定触发器的进入回调（通过 Predicate 检查触发器是否匹配）。
func (sd InternalStateDefinition) OnTriggerEntry(trigger plinko.Trigger, entryFn plinko.Operation, opts ...plinko.OperationOption) plinko.StateDefinition {
	sd.Callbacks.AddEntry(func(_ context.Context, _ plinko.Payload, t plinko.TransitionInfo) error {
		if t.GetTrigger() == trigger {
			return nil
		}
		return fmt.Errorf("trigger '%s' not found for entry", trigger)
	}, entryFn, newOperationConfig(entryFn, opts...))
	return sd
}

// OnTriggerExit 添加针对特定触发器的退出回调。
func (sd InternalStateDefinition) OnTriggerExit(trigger plinko.Trigger, exitFn plinko.Operation, opts ...plinko.OperationOption) plinko.StateDefinition {
	sd.Callbacks.AddExit(func(_ context.Context, _ plinko.Payload, t plinko.TransitionInfo) error {
		if t.GetTrigger() == trigger {
			return nil
		}
		return fmt.Errorf("trigger '%s' not found for exit", trigger)
	}, exitFn, newOperationConfig(exitFn, opts...))
	return sd
}

// PermitReentry 允许通过触发器重新进入当前状态（自循环）。
func (sd InternalStateDefinition) PermitReentry(trigger plinko.Trigger) plinko.StateDefinition {
	addPermit(&sd, trigger, sd.State, nil)
	return sd
}

// PermitReentryIf 带条件的自循环。
func (sd InternalStateDefinition) PermitReentryIf(predicate plinko.Predicate, trigger plinko.Trigger) plinko.StateDefinition {
	addPermit(&sd, trigger, sd.State, predicate)
	return sd
}

// Permit 定义到一个目标状态的转换。
func (sd InternalStateDefinition) Permit(trigger plinko.Trigger, destinationState plinko.State) plinko.StateDefinition {
	addPermit(&sd, trigger, destinationState, nil)
	return sd
}

// PermitIf 带守卫条件的转换。
func (sd InternalStateDefinition) PermitIf(predicate plinko.Predicate, trigger plinko.Trigger, destinationState plinko.State) plinko.StateDefinition {
	addPermit(&sd, trigger, destinationState, predicate)
	return sd
}

// PermitDynamic 定义动态转换，使用 resolver 在运行时决定目标状态。
func (sd InternalStateDefinition) PermitDynamic(trigger plinko.Trigger, defaultState plinko.State, resolver plinko.DynamicResolver) plinko.StateDefinition {
	addDynamicPermit(&sd, trigger, defaultState, resolver)
	return sd
}

// AbstractSyntax 是状态机的抽象语法树，包含所有状态、触发器和状态定义。
type AbstractSyntax struct {
	States             []plinko.State
	TriggerDefinitions []TriggerDefinition
	StateDefinitions   []*InternalStateDefinition
}

// PlinkoDefinition 是运行时定义结构，持有状态映射、副作用列表和抽象语法树。
type PlinkoDefinition struct {
	States      *map[plinko.State]*InternalStateDefinition
	SideEffects []sideeffects.SideEffectDefinition
	Abs         AbstractSyntax
}

// findDestinationState 检查状态列表中是否存在指定状态。
func findDestinationState(states []plinko.State, searchState plinko.State) bool {
	for _, searchVal := range states {
		if searchVal == searchState {
			return true
		}
	}
	return false
}

// SideEffect 注册全局副作用（所有阶段触发）。
func (pd *PlinkoDefinition) SideEffect(sideEffect plinko.SideEffect) plinko.PlinkoDefinition {
	pd.SideEffects = append(pd.SideEffects, sideeffects.SideEffectDefinition{Filter: sideeffects.AllowAllSideEffects, SideEffect: sideEffect})
	return pd
}

// FilteredSideEffect 注册仅在特定阶段触发的副作用。
func (pd *PlinkoDefinition) FilteredSideEffect(filter plinko.SideEffectFilter, sideEffect plinko.SideEffect) plinko.PlinkoDefinition {
	pd.SideEffects = append(pd.SideEffects, sideeffects.SideEffectDefinition{Filter: filter, SideEffect: sideEffect})
	return pd
}

// Configure 开始配置一个状态，若状态已存在则 panic。
func (pd *PlinkoDefinition) Configure(state plinko.State, opts ...plinko.StateOption) plinko.StateDefinition {
	if _, ok := (*pd.States)[state]; ok {
		panic(fmt.Sprintf("State: %s - has already been defined, plinko configuration invalid.", state))
	}
	cbd := composition.CallbackDefinitions{}
	sd := InternalStateDefinition{
		State:     state,
		Triggers:  make(map[plinko.Trigger]*TriggerDefinition),
		Abs:       &pd.Abs,
		Callbacks: &cbd,
		info:      newStateConfig(state, opts...),
	}
	(*pd.States)[state] = &sd
	pd.Abs.States = append(pd.Abs.States, state)
	pd.Abs.StateDefinitions = append(pd.Abs.StateDefinitions, &sd)
	return sd
}

// TriggerDefinition 定义一个触发器，包含目标状态和守卫条件等。
type TriggerDefinition struct {
	Name             plinko.Trigger
	DestinationState plinko.State
	Predicate        func(context.Context, plinko.Payload, plinko.TransitionInfo) error
	DynamicResolver  plinko.DynamicResolver
}

// PlinkoDataStructure 似乎是未使用的结构体，可能计划用于序列化。
type PlinkoDataStructure struct {
	States map[plinko.State]plinko.StateDefinition
}

// addDynamicPermit 为状态添加动态转换。
func addDynamicPermit(sd *InternalStateDefinition, trigger plinko.Trigger, defaultState plinko.State, resolver plinko.DynamicResolver) {
	if _, ok := sd.Triggers[trigger]; ok {
		panic(fmt.Sprintf("Trigger: %s - has already been defined", trigger))
	}
	td := TriggerDefinition{
		Name:             trigger,
		DestinationState: defaultState,
		DynamicResolver:  resolver,
	}
	sd.Triggers[trigger] = &td
	sd.Abs.TriggerDefinitions = append(sd.Abs.TriggerDefinitions, td)
}

// addPermit 为状态添加静态转换，可选守卫条件。
func addPermit(sd *InternalStateDefinition, trigger plinko.Trigger, destination plinko.State, predicate func(context.Context, plinko.Payload, plinko.TransitionInfo) error) {
	if _, ok := sd.Triggers[trigger]; ok {
		panic(fmt.Sprintf("Trigger: %s - has already been defined, plinko configuration invalid.", trigger))
	}
	td := TriggerDefinition{
		Name:             trigger,
		DestinationState: destination,
		Predicate:        predicate,
	}
	sd.Triggers[trigger] = &td
	sd.Abs.TriggerDefinitions = append(sd.Abs.TriggerDefinitions, td)
}

// newOperationConfig 构造操作配置，默认名称为函数全名。
func newOperationConfig(op interface{}, opts ...plinko.OperationOption) plinko.OperationConfig {
	c := plinko.OperationConfig{
		Name: getFunctionName(op),
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// getFunctionName 获取函数的完全限定名。
func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// newStateConfig 构造状态配置，默认名称为状态字符串。
func newStateConfig(state plinko.State, opts ...plinko.StateOption) plinko.StateConfig {
	c := plinko.StateConfig{
		Name: string(state),
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}
