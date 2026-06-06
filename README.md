# Plinko - a Fluent State Machine for Go
## forked from [clackiverse/plinko-1](https://github.com/clackiverse/plinko-1)

[![Build Status](https://drone.shipt.com/api/badges/shipt/plinko/status.svg)](https://drone.shipt.com/shipt/plinko) [![codecov](https://codecov.io/gh/shipt/plinko/branch/main/graph/badge.svg?token=8UX649KGGV)](https://codecov.io/gh/shipt/plinko) Build Status

## Create state machines and lightweight state machine-based workflows directly in golang code

The project, as well as the example below, are inspired by the [Erlang State Machine](https://erlang.org/doc/design_principles/statem.html) and [Stateless project](https://github.com/dotnet-state-machine/stateless) implementations. The goal is to create the fastest state machine that can be reused across many entities with the least amount of overhead in the process.

## Why State Machines?
Some state machine implementations keep track of an in-memory state during the running of an application. This makes sense for desktop applications or games where the journey of that state is critical to the user-facing process, but that doesn't map well to web services shepherding things like Orders and Products that number in the thousands-to-millions on any given day.

This allows the state machine to be reduced to a simple data structure, and enables the cost of wiring up the machine to happen only once but reused multiple times.  In turn, the state machine can be shared across multiple threads and executed concurrently without interference between discrete runs.

There are a number of good articles on this front, there are a couple that focus on state design from the [esoteric around soundness of the design](https://en.wikibooks.org/wiki/Haskell/Understanding_monads/State) to the more [functional programming based definition of a state machine](https://hexdocs.pm/as_fsm/readme.html).

## Common implementation pattern in web services
Many times, a web service may have controllers that span the lifecycle of the entity they are coordinating.  This pattern allows the controller to play the key, narrow role of traffic coordinator and defers execution decisions to the state machine.  The state machine introduces two key notions: State and Trigger.  Triggers are mapped to states and execution paths can be different based on states. Applying this to an MVC pattern, the entity contains the state and the state modifying `[POST|PUT|PATCH]` endpoint is the trigger.  For example:

An order can be in different states during its lifecycle:  Open, Claimed, Delivered, etc.   If someone wishes to cancel that order, there are different protocols and processes involved in each of those states.  In this approach a `/cancel/{id}` endpoint is called.  The controller loads the order into a payload and fires the `Cancel` trigger at it using the state machine.  The state machine selects the proper flow and returns the status when complete.

## Features

* Simple support for states and triggers
* Entry/Exit events for states
* Side Effect support for supporting uniform functionality when modifying state
* Error events used to properly respond to errors raised during a state transition

Some useful extensions are also provided:

* Pushes state external to the structure - instantiate once, use many times.
* Reentrant states
* Export to PlantUML


# Installing 

Using Plinko is easy.   First, use `go get` to install the latest version of the library.  This command will install everything you need - in fact, one design goal of Plinko is to minimize dependencies.  There are no runtime dependencies required for Plinko, and the only dependencies used by the project are used for unit testing.

```
go get -u github.com/drkisler/plinko
```

Next, include Plinko in your application:

```
import "github.com/drkisler/plinko"
```

You will define state machine using the examples below, and compiling the state machine once to reuse again and again.  Efficiency is front of mind,  meaning the compilation process is fast and runs in far less than 1/10,000th of a second on a reasonable VM. Or, given a single thread on an x86 processor, a statemachine can be fully compiled and ready to run more than 10,000,000 times a second.

## License
shipt/plinko is licensed under the [MIT license](./LICENSE.md).

# Introspection
The state machine can provide a list of triggers for a given state to provide simple access to the list of triggers for any state.

## Creating a state machine
A state machine is created by articulating the states,  the triggers that can be used at each state and the destination state where they land. Here is a sample declaration of the states and triggers we will use:

```go
package shomething
type State string
const Created          State = "Created"
const Opened           State = "Opened"
const Claimed          State = "Claimed"
const ArriveAtStore    State = "ArrivedAtStore"
const MarkedAsPickedUp State = "MarkedAsPickedup"
const Delivered        State = "Delivered"
const Canceled         State = "Canceled"
const Returned         State = "Returned"

type Trigger string
const Submit    Trigger = "Submit"
const Cancel    Trigger = "Cancel"
const Open      Trigger = "Open"
const Claim     Trigger = "Claim"
const Deliver   Trigger = "Deliver"
const Return    Trigger = "Return"
const Reinstate Trigger = "Reinstate"
```

Below, a state machine is created describing a set of states an order can progress through along with the triggers that can be used.

```
p := config.CreatePlinkoDefinition()

p.Configure(Created).
   OnEntry(OnNewOrderEntry).
   Permit(Open, Opened).
   Permit(Cancel, Canceled)

p.Configure(Opened).
   Permit(AddItemToOrder, Opened).
   Permit(Claim, Claimed).
   Permit(Cancel, Canceled)

p.Configure(Claimed).
   Permit(AddItemToOrder, Claimed).
   Permit(Submit, ArriveAtStore).
   Permit(Cancel, Canceled)

p.Configure(ArriveAtStore).
   Permit(Submit, MarkedAsPickedUp).
   Permit(Cancel, Canceled)

p.Configure(MarkedAsPickedUp).
   Permit(Deliver, Delivered).
   Permit(Cancel, Canceled)

p.Configure(Delivered).
   Permit(Return, Returned)

p.Configure(Canceled).
   Permit(Reinstate, Created)
	
p.Configure(Returned)
```
Add new function PermitDynamic.
```
p.Configure(FinancePending).
  PermitDynamic(Approve,Approved,DetermineFinanceRoute).
  Permit(Reject,Rejected)



func DetermineFinanceRoute(ctx context.Context,payload plinko.Payload,info plinko.TransitionInfo)(plinko.State,error) {
  expense := payload.(*Expense)
  if expense.Amount >= 5000 {

    return DirectorPending,nil
  }

  return Approved,nil
}



```

Once created, the next step is compiling the state machine.  This means the state machine is validated for complete-ness.  At this stage, Errors and Warnings are raised.  This incidentally allows the state machine definition to be fully testable in the build pipeline before deployment.

```
co := p.Compile()

if co.error {
   // exit
}

fsm := co.StateMachine
```

Once we have the state machine, we can pass that around explicitly or through things like controller context to make it available where needed.

We can trigger the state processes by creating a PlinkoPayload and handing it to the statemachine like so:

```
payload := appPayload{ /* ... */ }
fsm.Fire(ctx, appPayload, Submit)
```

## Permitted Transitions

The state machine allows the definitions of transitions using the `Permit` function.  This means I can declare that a triggered action can happen on one state, but not another using:

```
p.Configure(Opened).
   // ...
   Permit(Cancel, Canceled)

p.Configure(Claimed).
   // The key here is that Canceling from a Claimed state is not permitted.
```

This is useful, because I now have guard rails around when a `Cancel` trigger can be used and when it cannot.  Furthermore, I can use the `CanFire()` method of the state machine to ask if I have a valid action:

```
if !fsm.CanFire(ctx, payload, Cancel) {
   return "Cannot perform this action"
}
```

Furthermore, let's say `Cancel` is allowed within a timeframe described in the payload.  In this case, let's say it's valid when our order is more than 1 hour from being scheduled to shop.  In this, we first define a predicate function:

```
func IsOrderCancellable(p Payload, t TransitionInfo) bool {
   return p.ScheduledToShop().Sub(time.Now()).Hours() >= 1
}
```

In this case, I define the trigger differently with:

```
p.Configure(Claimed).
   PermitIf(IsOrderCancellable, Cancel, Cancelled)
```

Using `PermitIf` now allows the `fsm.CanFire` code block above to be executed without modification,  but now the state machine validates if the trigger can be used based on the order's scheduled to shop time.

### Reentrancy
Reentrancy is a state transition where the destination is the same State.   This means `OnExit` functions get called for the current state, followed by the `OnEntry` calls for the current state.  All the SideEffects are also accordingly raised as expected with the source and destination states being the same.

A simple reentrant state is defined as:

```
p.Configure(Claimed).
   PermitReentry(AddItemToOrder)
```

Likewise, a conditional `PermitReentryIf` can be defined that relies on a predicate to decide if the trigger may be fired.  For this example, a rule might determine that an item can only be added based on timing or other conditions described in a function called `ItemAddRule`.

```
p.Configure(Claimed).
   PermitReentryIf(ItemAddRule, AddItemToOrder)
```   

## Functional Composition

When entering or exiting a state, a series of functions need to act to make that transition complete.  Some transitions are simple, and some are complex.  The key here is creating a series of steps that are testable and operate based on a standard pattern.

Let's take a look at a piece of code we setup earlier:


```
p.Configure(Created).
   OnEntry(OnNewOrderEntry).
   Permit(Open, Opened).
   Permit(AddItem, Created)
```

OnNewOrderEntry is function defined as such:

```
func OnNewOrderEntry(p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
   // perform a series of steps based on the 
   // payload and transition info
   // ...

   return p, nil
}
```

This is useful for a couple of reasons: First, this becomes one distinct action that can succeed or fail.  When it succeeds, the chain continues and works toward the successful transition to the new state. And second, this is an operation that can be tested in isolation.

Both of these reasons are significant when building a complex set of transitions.

Next, we have a variation on the chaining where we can say "only run this function if a particular trigger triggered the transition".   This is the `OnTriggerEntry(trigger, func)` function.

```
p.Configure(Created).
   OnTriggerEntry(AddItem, RecalculateTotals).
   Permit(Open, Opened).
   Permit(AddItem, Created)
```

In the example above, the `RecalculateTotals` function is only executed when the `AddItem` trigger is raised.   This allows us to explicitly describe the transition steps without placing that complexity inside the `RecalculateTotals` function.


## Side-Effect Support

Side-Effect supports wiring up common functions that respond to state changes happening.   This is a great place for logging and recording movement in a uniform way.

Side Effects are raised at different phases of a state transition.  Given an order that's sitting in a `Created` state that has been actioned with an `Open` trigger, we'll see the following calls to the SideEffect functions.

| State | Action |  Trigger | Destination State |
| --- | --- | --- | --- |
| Created | BeforeTransition | Open | Opened |
| Created | BetweenStates | Open | Opened |
| Created | AfterTransition | Open | Opened |

In the above list, you can see the registered function is called 4 times throughout the lifecycle of the transition.   This gives us consistency and observability throughout the process.

We can better understand how this works by looking at a standard configuration.

```
// given a standard definition ...
p := plinko.CreateDefinition()

p.Configure(Created).
   OnEntry(OnNewOrderEntry).
   Permit(Open, Opened).
   Permit(Cancel, Canceled)

p.Configure(Opened).
   Permit(AddItemToOrder, Opened).
   Permit(Claim, Claimed).
   Permit(Cancel, Canceled)


// we register for side effects like this.
p.SideEffect(StateLogging)
p.SideEffect(MetricsRecording)
p.FilteredSideEffect(AfterStateEntry, StateEntryRecording)
```

In the case above, we registered two functions that get executed whenever a change happens.  These functions will always be called in the order they are registered for a given state transition.

In addition, we registered a FilteredSideEffect that only gets called on the requested action.

These are functions that have signature including the starting state, the destination state, the trigger used to kick off the transition and the payload.

```
func StateLogging(action StateAction, payload Payload, transitionInfo TransitionInfo) {
   // this can typically be broken out into a function on the logger, but keeping
   // it here for clarity in demonstration

   logEntry := StateLog {
      Action:           action,
      SourceState:      transitionInfo.GetSource(),
      DestinationState: transitionInfo.GetDestination(),
      Trigger:          transitionInfo.GetTrigger(),
      OrderID:          payload.GetOrderID(),
   }

   // call to our logger that will decorate the entry with timing information and the like.
   logger.LogStateInfo(logEntry)
}

func MetricsRecording(action StateAction, payload Payload, transitionInfo TransitionInfo) {
   // this can be a simple function that pulls apart the details and sends them to
   // things like graphite, influx or any timeseries metrics database for graphing and alerting.
   metrics.RecordStateMovement(action, payload, transitionInfo)
}

```


## Error Handling

State Machine error handling follows the same pattern that we see in golang in general, when an error occurs that cannot be rectified and causes the state change to fail, an error is raised from the function.   Plinko redirects the flow to the `OnError` definition for remediation. An error in this situation can mean that a Payloads state is moved to something other than the original destination.  Depending on the system, this might be mean it goes back to an old state, continues on to the new state or it lands in a _triage_ state.  Equally important is that this information can be recorded reliably with the Side-Effect support documented above.  Plinko ensures the ability to adjust the destination state and make that consistent with SideEffects.

While the `OnEntry` and `OnExit` function definitions take a `TransitionInfo` parameter that is immutable, and error operation is defined with a `ModifiableTransitionInfo` interface that allows the function to change the `DestinationState`.  In addition, the function also accepts the error raised during the `On[Entry|Exit]` operation so it can be interrogated when necessary.  The definition of an error operation handler looks like this:

```
 ErrorOperation func(Payload, ModifiableTransitionInfo, error) (Payload, error)
```

An ErrorOperation function implements this signature and tests the error case.  Here is an example where we redirect based on a match.

```
func RedirectOnDeactivatedCustomer(p Payload, m ModifiableTransitionInfo, e error) (Payload, error) {
   if e == DeactivatedCustomerError {
      m.SetDestination(DeactivatedTriage)
      return RecordOrder(p, m)
   }

   // we didn't identify the error, so we'll pass this through for further error handling
   return p, nil
}
```

There are a couple things to note.   If you return a non-nil `error` during an `OnError` routine, this is regarded as a fatal error that is floated to the caller who initiated the `.Fire(..)` command.  This condition is floated to the registered SideEffect handlers as well.

Some key pieces to remember when building up a set of error handlers.    First, you don't have to handle _every_ error case.  This is done by returning `(payload, nil)` to the caller.  Plinko will call any subsequent error handlers in this case to give each handler an opportunity to perform its role in the set of operations.  This is powerful, as handlers can take on different aspects of error handling, including custom messaging and metrics. This allows these functions to be simple, focused operations that compose a larger set of responsibilities (through additional functions) when an error occurs.

Lastly, here is a sample plinko configuration that uses error handling to perform the proper state destination redirect shown above when an order transitions to `Opened` and the user has been deactivated.  Note the separation of concerns - one to perform the redirect and save state, and the other to perform a system notification.

```
p.Configure(Created).
   Permit(Open, Opened).
   Permit(Cancel, Canceled)

p.Configure(Opened).
   OnEntry(OnOrderOpen).
   OnError(RedirectOnDeactivatedCustomer).
   OnError(GenerateSlackMessageNotification).
   Permit(AddItemToOrder, Opened).
   Permit(Claim, Claimed).
   Permit(Cancel, Canceled)
```

## Panic Support
On calls to Entry or Exit Functions, Plinko will capture any panics.  These panics are recorded as a structured error, containing when and where the error occured.  The `OnError` handlers can then respond as appropriate.

## State Machine self-documentation
The fsm can document itself upon a successful compile - emitting PlantUML which can, in turn, be rendered into a state diagram:

```
uml, err := p.RenderUml()

if err != nil {
   // exit...
}

fmt.Println(string(uml))
```

![PlantUML Rendered State Diagram](./docs/sample_state_diagram.png)


# 补充

## 状态与触发器的定义及配置

### 1. 创建状态机定义

```
import "github.com/drkisler/plinko/pkg/config"

pd := config.CreatePlinkoDefinition()
```

`CreatePlinkoDefinition()` 初始化一个内部的 `PlinkoDefinition`，持有状态映射表和抽象语法树。

---

### 2. 配置状态（Configure）

```
pd.Configure(plinko.State("Created"),
    state.WithName("已创建"),
    state.WithDescription("订单已创建，等待支付"),
)
```

**⚠️ 同一个 State 不能重复 Configure，否则直接 panic：**

```
// internal/runtime/internal.go
if _, ok := (*pd.States)[state]; ok {
    panic(fmt.Sprintf("State: %s - has already been defined...", state))
}
```

---

### 3. 配置触发器（Permit 系列）

| 方法 | 说明 |
|---|---|
| `Permit(trigger, dest)` | 无条件跳转 |
| `PermitIf(predicate, trigger, dest)` | 带条件跳转，predicate 返回 error 时阻断 |
| `PermitReentry(trigger)` | 重入当前状态 |
| `PermitReentryIf(predicate, trigger)` | 带条件重入 |
| `PermitDynamic(trigger, defaultDest, resolver)` | 运行时动态决定目标状态 |

**⚠️ 同一状态内同一个 Trigger 不能重复定义，否则 panic：**

```
// addPermit / addDynamicPermit
if _, ok := sd.Triggers[trigger]; ok {
    panic(fmt.Sprintf("Trigger: %s - has already been defined...", trigger))
}
```

---

### 4. 配置回调（OnEntry / OnExit / OnError）

```
pd.Configure(plinko.State("Paid")).
    OnEntry(handlePaymentEntry).
    OnTriggerEntry("Cancel", handleCancelEntry).  // 仅特定触发器触发时执行
    OnExit(handlePaidExit).
    OnError(handleError).
    Permit("Ship", plinko.State("Shipped")).
    Permit("Cancel", plinko.State("Cancelled"))
```

回调执行顺序（见 `trigger.go` 的 `Fire` 方法）：

```
BeforeTransition SideEffect
    → 源状态 ExitChain（OnExit / OnTriggerExit）
        → 若 Exit 出错 → ErrorChain → 返回
    → BetweenStates SideEffect
    → 目标状态 EntryChain（OnEntry / OnTriggerEntry）
        → 若 Entry 出错 → ErrorChain → 返回
    → AfterTransition SideEffect
```

---

## 如何避免 Compile() 的 panic、警告和错误

### Panic（运行时直接崩溃，编译期无法检测）

**原因一：重复 Configure 同一状态**
```
// ❌ 会 panic
pd.Configure("Created")
pd.Configure("Created") // panic!

// ✅ 每个状态只配置一次
created := pd.Configure("Created")
```

**原因二：同一状态重复定义同一 Trigger**
```
// ❌ 会 panic
pd.Configure("Created").
    Permit("Pay", "Paid").
    Permit("Pay", "Cancelled") // panic! Pay 已定义
```

**原因三：回调链中 panic（被 recover 捕获，转为 PlinkoError）**

`executeChain` 和 `executeErrorChain` 内部用 `defer recover()` 包裹，回调里的 panic 会被转换成带堆栈的 `PlinkoPanicError` 返回，不会崩溃程序，但需要在 `Fire()` 的返回值中检查 error。

---

### CompileError（Compile() 返回，不崩溃但状态机不可用）

**原因：Trigger 指向了未定义的目标状态**

```
// ❌ "Shipped" 从未被 Configure，编译报错
pd.Configure("Paid").
    Permit("Ship", "Shipped") // CompileError: State 'Shipped' undefined

// ✅ 确保所有 DestinationState 都被 Configure
pd.Configure("Paid").Permit("Ship", "Shipped")
pd.Configure("Shipped").Permit("Complete", "Done")  // Shipped 必须存在
pd.Configure("Done")                                // Done 也必须存在
```

`PermitDynamic` 是例外——动态目标在运行时解析，Compile 阶段跳过检查：

```
// compiler.go
if def.DynamicResolver != nil {
    continue  // 动态路由不做编译期检查
}
```

---

### CompileWarning（Compile() 返回，状态机仍可用但逻辑可能有误）

**原因：某个状态没有任何 Trigger（死端状态）**

```
// ⚠️ Warning: 'Done' is a state without any triggers (deadend state)
pd.Configure("Done")

// 如果这是故意的终态，可以忽略此警告
// 可以通过检查 Messages 过滤处理：
co := pd.Compile()
for _, msg := range co.Messages {
    if msg.CompileMessage == plinko.CompileError {
        // 必须处理
        log.Fatal(msg.Message)
    }
    if msg.CompileMessage == plinko.CompileWarning {
        // 终态死端可以接受，按业务判断
        log.Warn(msg.Message)
    }
}
```

---

## 完整的安全使用模板

```
package main

import (
	"context"
	"log"

	"github.com/drkisler/plinko"
	"github.com/drkisler/plinko/pkg/config"
	"github.com/drkisler/plinko/pkg/config/state"
)

// ─── 状态常量 ────────────────────────────────────────────────────────────────

const (
	StateCreated   plinko.State = "Created"
	StatePaid      plinko.State = "Paid"
	StateShipped   plinko.State = "Shipped"
	StateDone      plinko.State = "Done"
	StateCancelled plinko.State = "Cancelled"
)

// ─── 触发器常量 ───────────────────────────────────────────────────────────────

const (
	TriggerPay    plinko.Trigger = "Pay"
	TriggerShip   plinko.Trigger = "Ship"
	TriggerFinish plinko.Trigger = "Finish"
	TriggerCancel plinko.Trigger = "Cancel"
)

// ─── Payload ─────────────────────────────────────────────────────────────────

type OrderPayload struct {
	OrderID string
	State   plinko.State
}

func (o *OrderPayload) GetState() plinko.State {
	return o.State
}

// ─── 回调函数 ─────────────────────────────────────────────────────────────────

func onPaidEntry(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	order := p.(*OrderPayload)
	order.State = t.GetDestination() // 必须手动更新状态
	log.Printf("[Entry] 订单 %s 已支付，来自状态: %s", order.OrderID, t.GetSource())
	return order, nil
}

func onShippedEntry(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	order := p.(*OrderPayload)
	order.State = t.GetDestination()
	log.Printf("[Entry] 订单 %s 已发货", order.OrderID)
	return order, nil
}

func onCancelledEntry(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	order := p.(*OrderPayload)
	order.State = t.GetDestination()
	log.Printf("[Entry] 订单 %s 已取消，触发器: %s", order.OrderID, t.GetTrigger())
	return order, nil
}

func onCreatedExit(ctx context.Context, p plinko.Payload, t plinko.TransitionInfo) (plinko.Payload, error) {
	log.Printf("[Exit] 离开 Created 状态，目标: %s", t.GetDestination())
	return p, nil
}

func onErrorHandler(ctx context.Context, p plinko.Payload, t plinko.ModifiableTransitionInfo, err error) (plinko.Payload, error) {
	log.Printf("[Error] 状态转换出错 %s -> %s，原因: %v",
		t.GetSource(), t.GetDestination(), err)
	// 返回 nil 表示错误已处理，继续流程；返回 err 则向上传递
	return p, err
}

func onSideEffect(ctx context.Context, action plinko.StateAction, p plinko.Payload, t plinko.TransitionInfo, elapsedMs int64) {
	log.Printf("[SideEffect] action=%s %s -> %s trigger=%s elapsed=%dms",
		action, t.GetSource(), t.GetDestination(), t.GetTrigger(), elapsedMs)
}

// ─── 构建状态机 ───────────────────────────────────────────────────────────────

func buildStateMachine() (plinko.StateMachine, error) {
	pd := config.CreatePlinkoDefinition()

	// 注册全局 SideEffect（可选）
	pd.SideEffect(onSideEffect)

	// Created：初始状态，可支付或取消
	pd.Configure(StateCreated,
		state.WithName("已创建"),
		state.WithDescription("订单已创建，等待支付"),
	).
		OnExit(onCreatedExit).
		OnError(onErrorHandler).
		Permit(TriggerPay, StatePaid).
		Permit(TriggerCancel, StateCancelled)

	// Paid：已支付，可发货或取消
	pd.Configure(StatePaid,
		state.WithName("已支付"),
		state.WithDescription("订单已支付，等待发货"),
	).
		OnEntry(onPaidEntry).
		OnError(onErrorHandler).
		Permit(TriggerShip, StateShipped).
		Permit(TriggerCancel, StateCancelled)

	// Shipped：已发货，可完成
	pd.Configure(StateShipped,
		state.WithName("已发货"),
		state.WithDescription("订单已发货，等待签收"),
	).
		OnEntry(onShippedEntry).
		OnError(onErrorHandler).
		Permit(TriggerFinish, StateDone)

	// Done：终态，标记 AsTerminal 消除 Warning
	pd.Configure(StateDone,
		state.WithName("已完成"),
		state.WithDescription("订单已完成"),
		state.AsTerminal(),
	)

	// Cancelled：终态，标记 AsTerminal 消除 Warning
	pd.Configure(StateCancelled,
		state.WithName("已取消"),
		state.WithDescription("订单已取消"),
		state.AsTerminal(),
	).
		OnEntry(onCancelledEntry).
		OnError(onErrorHandler)

	// ── 编译并检查 ──────────────────────────────────────────────────────────
	co := pd.Compile()

	for _, msg := range co.Messages {
		switch msg.CompileMessage {
		case plinko.CompileError:
			// 有错误直接返回，状态机不可用
			return nil, fmt.Errorf("状态机编译错误: %s", msg.Message)
		case plinko.CompileWarning:
			// 经过 AsTerminal 标记后，正常情况下不应再出现 Warning
			// 若仍出现，说明有非预期的死端状态，记录日志
			log.Printf("[Warning] 状态机编译警告: %s", msg.Message)
		}
	}

	return co.StateMachine, nil
}

// ─── 主流程 ───────────────────────────────────────────────────────────────────

func main() {
	sm, err := buildStateMachine()
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	ctx := context.Background()
	order := &OrderPayload{
		OrderID: "ORD-001",
		State:   StateCreated,
	}

	// 检查是否可以触发
	if err := sm.CanFire(ctx, order, TriggerPay); err != nil {
		log.Fatalf("不可触发: %v", err)
	}

	// 触发状态转换
	payload, err := sm.Fire(ctx, order, TriggerPay)
	if err != nil {
		log.Fatalf("触发失败: %v", err)
	}
	order = payload.(*OrderPayload)
	log.Printf("当前状态: %s", order.GetState()) // Paid

	// 枚举当前可用触发器
	triggers, err := sm.EnumerateActiveTriggers(order)
	if err != nil {
		log.Fatalf("枚举失败: %v", err)
	}
	log.Printf("可用触发器: %v", triggers) // [Ship Cancel]
}
```

几个要点提示一下：

**`GetState()` 返回值依赖手动更新**，plinko 不会自动修改 Payload 里的状态字段，必须在 `OnEntry` 回调里执行 `order.State = t.GetDestination()`，否则下一次 `Fire` 会找不到正确的状态定义。

**终态配置了 `OnEntry` 也完全合法**，如 `StateCancelled` 示例所示，`AsTerminal` 只影响编译期的警告检查，不影响回调执行。

**`OnError` 返回值决定错误传播行为**：返回原 `err` 则调用方的 `Fire` 收到错误；返回 `nil` 则错误被吞掉，`Fire` 正常返回，需根据业务选择。

核心原则：**所有 `Permit` 指向的状态必须被 `Configure`；每个状态和触发器组合只定义一次；`Compile()` 的结果必须检查 `CompileError`。**