// plinko/internal/runtime/compiler.go

/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package runtime

import (
	"bytes"
	"fmt"

	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/internal/renderers"
)

// Compile 编译 PlinkoDefinition，检查未定义的目标状态和死胡同状态（无触发器），
// 生成 CompilerOutput，包含状态机实例。
func (pd PlinkoDefinition) Compile() plinko.CompilerOutput {
	var compilerMessages []plinko.CompilerMessage

	// 检查所有非动态触发器指向的目标状态是否存在
	for _, def := range pd.Abs.TriggerDefinitions {
		if def.DynamicResolver != nil {
			continue // 动态解析的触发器不在此校验
		}
		if !findDestinationState(pd.Abs.States, def.DestinationState) {
			compilerMessages = append(compilerMessages, plinko.CompilerMessage{
				CompileMessage: plinko.CompileError,
				Message:        fmt.Sprintf("State '%s' undefined: Trigger '%s' declares a transition to this undefined state.", def.DestinationState, def.Name),
			})
		}
	}

	// 警告那些没有任何触发器的状态（死胡同）
	for _, def := range pd.Abs.StateDefinitions {
		if def.info.Terminal {
			continue // 终态不检查 Trigger
		}
		if len(def.Triggers) == 0 {
			compilerMessages = append(compilerMessages, plinko.CompilerMessage{
				CompileMessage: plinko.CompileWarning,
				Message:        fmt.Sprintf("State '%s' is a state without any triggers (deadend state).", def.State),
			})
		}
	}

	psm := plinkoStateMachine{
		pd: pd,
	}

	co := plinko.CompilerOutput{
		Messages:     compilerMessages,
		StateMachine: psm,
	}
	return co
}

// RenderUml 编译并渲染 UML，如果有编译错误则返回错误。
func (pd PlinkoDefinition) RenderUml() (plinko.Uml, error) {
	cm := pd.Compile()
	for _, def := range cm.Messages {
		if def.CompileMessage == plinko.CompileError {
			return "", fmt.Errorf("critical errors exist in definition")
		}
	}
	b := bytes.NewBuffer([]byte{})
	r := renderers.NewUML(b)
	err := pd.Render(r)
	return plinko.Uml(b.String()), err
}

// Render 使用指定的渲染器渲染图。
func (pd PlinkoDefinition) Render(renderer plinko.Renderer) error {
	return renderer.Render(pd)
}

// Edges 实现 plinko.Graph 接口，遍历所有转换边。
func (pd PlinkoDefinition) Edges(edgeFunc func(state, destinationState plinko.State, name plinko.Trigger)) {
	for _, sd := range pd.Abs.StateDefinitions {
		for _, td := range sd.Triggers {
			edgeFunc(sd.State, td.DestinationState, td.Name)
		}
	}
}

// Nodes 实现 plinko.Graph 接口，遍历所有状态节点。
func (pd PlinkoDefinition) Nodes(nodeFunc func(state plinko.State, StateConfig plinko.StateConfig)) {
	for _, sd := range pd.Abs.StateDefinitions {
		nodeFunc(sd.State, sd.info)
	}
}
