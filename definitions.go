// plinko/definitions.go

package plinko

import (
	"context"
)

// State 表示状态机中的一个状态，本质是一个字符串别名。
type State string

// Trigger 表示触发状态转换的触发器，本质是一个字符串别名。
type Trigger string

// Predicate 是守卫条件函数，在转换前检查是否允许执行。
// 返回 error 表示条件不满足。
type Predicate func(context.Context, Payload, TransitionInfo) error

// TriggerPredicate 是另一种谓词，返回布尔值（代码中未直接使用，作为扩展预留）。
type TriggerPredicate func(context.Context, Payload, TransitionInfo) bool

// Operation 是状态回调（OnEntry/OnExit）以及触发器回调的函数签名。
// 接收上下文、载荷和转换信息，返回可能修改后的载荷和错误。
type Operation func(context.Context, Payload, TransitionInfo) (Payload, error)

// ErrorOperation 是错误处理回调的函数签名，可在转换出错时被调用。
// 额外接收一个 error 参数，且转换信息是可修改的（ModifiableTransitionInfo），
// 允许在错误恢复中改变目标状态。
type ErrorOperation func(context.Context, Payload, ModifiableTransitionInfo, error) (Payload, error)

// DynamicResolver 用于动态决定目标状态，根据运行时上下文和载荷计算下一个状态。
type DynamicResolver func(context.Context, Payload, TransitionInfo) (State, error)

// StateDefinition 提供了流畅接口来配置一个状态的各种行为。
type StateDefinition interface {
	OnEntry(Operation, ...OperationOption) StateDefinition                 // 进入状态时执行
	OnError(ErrorOperation, ...OperationOption) StateDefinition            // 转换出错时执行
	OnExit(Operation, ...OperationOption) StateDefinition                  // 离开状态时执行
	OnTriggerEntry(Trigger, Operation, ...OperationOption) StateDefinition // 特定触发器导致的进入
	OnTriggerExit(Trigger, Operation, ...OperationOption) StateDefinition  // 特定触发器导致的离开
	Permit(Trigger, State) StateDefinition                                 // 允许从本状态经由触发器转换到目标状态
	PermitIf(Predicate, Trigger, State) StateDefinition                    // 带守卫条件的允许转换
	PermitReentry(Trigger) StateDefinition                                 // 允许自循环（重新进入本状态）
	PermitReentryIf(Predicate, Trigger) StateDefinition                    // 带条件的自循环
	PermitDynamic(Trigger, State, DynamicResolver) StateDefinition         // 动态目标转换
}

// StateMachine 是编译后的状态机，可执行触发转换等操作。
type StateMachine interface {
	Fire(context.Context, Payload, Trigger) (Payload, error)    // 执行一次转换
	CanFire(context.Context, Payload, Trigger) error            // 检查是否可以触发（条件满足）
	EnumerateActiveTriggers(payload Payload) ([]Trigger, error) // 获取当前状态所有可能的触发器
}

// TransitionInfo 提供只读的转换信息（源状态、目标状态、触发器）。
type TransitionInfo interface {
	GetSource() State
	GetDestination() State
	GetTrigger() Trigger
}

// ModifiableTransitionInfo 是可修改的转换信息，允许在错误处理中修改目标状态。
type ModifiableTransitionInfo interface {
	TransitionInfo
	SetDestination(State)
}

// SideEffect 是副作用函数，在转换的不同阶段被调用。
// 参数包括上下文、状态动作阶段、载荷、转换信息、已耗时（毫秒）。
type SideEffect func(context.Context, StateAction, Payload, TransitionInfo, int64)

// PlinkoDefinition 是构建状态机定义的顶级接口，提供配置状态和副作用。
type PlinkoDefinition interface {
	Configure(State, ...StateOption) StateDefinition                  // 开始配置一个状态
	SideEffect(SideEffect) PlinkoDefinition                           // 注册全局副作用（所有阶段）
	FilteredSideEffect(SideEffectFilter, SideEffect) PlinkoDefinition // 注册特定阶段副作用
	Compile() CompilerOutput                                          // 编译定义，生成状态机实例和编译消息
	RenderUml() (Uml, error)                                          // 输出 PlantUML 文本
	Render(Renderer) error                                            // 使用自定义渲染器输出图形
}

// Renderer 是图形渲染器接口（如 DOT、UML）。
type Renderer interface {
	Render(Graph) error
}

// Graph 是状态机图结构的抽象，供渲染器遍历节点和边。
type Graph interface {
	Edges(func(State, State, Trigger)) // 遍历所有转换边
	Nodes(func(State, StateConfig))    // 遍历所有状态节点
}

// Payload 是状态机载荷接口，必须能返回当前状态。
type Payload interface {
	GetState() State
}

// CompilerMessage 携带编译期间的消息（错误或警告）。
type CompilerMessage struct {
	CompileMessage CompilerReportType
	Message        string
}

// CompilerReportType 是编译消息类型。
type CompilerReportType string

const (
	CompileError   CompilerReportType = "Compile Error"
	CompileWarning CompilerReportType = "Compile Warning"
)

// StateAction 表示状态转换的不同阶段。
type StateAction string

const (
	BeforeTransition StateAction = "BeforeTransition" // 转换前（离开源状态后，进入目标状态前）
	BetweenStates    StateAction = "MiddleTransition" // 中间状态（执行离开回调后，进入回调前）
	AfterTransition  StateAction = "AfterTransition"  // 转换后（已进入目标状态）
)

// SideEffectFilter 用位掩码控制副作用在哪些阶段触发。
type SideEffectFilter int

const (
	AllowBeforeTransition SideEffectFilter = 1
	AllowBetweenStates    SideEffectFilter = 2
	AllowAfterTransition  SideEffectFilter = 4
)

type Uml string // UML 文本表示

// CompilerOutput 是编译结果，包含可执行的状态机和编译消息。
type CompilerOutput struct {
	StateMachine StateMachine
	Messages     []CompilerMessage
}

// OperationConfig 是操作回调的配置（如名称，用于调试/日志）。
type OperationConfig struct {
	Name string
}
type OperationOption func(c *OperationConfig)

// StateConfig 是状态的配置（名称、描述）。
type StateConfig struct {
	Name        string
	Description string
	Terminal    bool
}
type StateOption func(c *StateConfig)
