// plinko/internal/renderers/uml.go

/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package renderers

import (
	"fmt"
	"io"

	"github.com/drkisler/plinko"
)

// UML 渲染器，生成 PlantUML 格式。
type UML struct {
	*writeWrapper
}

// NewUML 创建 UML 渲染器。
func NewUML(w io.Writer) *UML {
	return &UML{
		writeWrapper: &writeWrapper{writer: w},
	}
}

// Render 输出 PlantUML 文本。
func (d *UML) Render(graph plinko.Graph) error {
	d.write([]byte("@startuml\n"))

	firstEdge := true
	graph.Edges(func(state, destinationState plinko.State, name plinko.Trigger) {
		// 第一个边前的注释：指向初始状态（这里简单地将第一个边的源状态作为起始）
		if firstEdge {
			d.write([]byte(fmt.Sprintf("[*] -> %s \n", state)))
			firstEdge = false
		}
		d.write([]byte(fmt.Sprintf("%s --> %s : %s\n", state, destinationState, name)))
	})

	d.write([]byte("@enduml"))
	return nil
}
