// plinko/internal/renderers/dot.go
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
	"os/exec"

	"github.com/drkisler/plinko"
)

// Dot 渲染器，输出 DOT 格式。
type Dot struct {
	*writeWrapper               // 带错误缓存的写入器
	style         dotStylesheet // DOT 样式模板
}

// NewDot 创建一个新的 DOT 渲染器。
func NewDot(w io.Writer) *Dot {
	return &Dot{
		writeWrapper: &writeWrapper{writer: w},
		style:        defaultDotStyle,
	}
}

// Render 将图写入 DOT 格式。
func (d *Dot) Render(graph plinko.Graph) error {
	d.beginGraph()
	// 遍历所有节点，写入节点定义
	graph.Nodes(func(state plinko.State, info plinko.StateConfig) {
		d.node(string(state), info.Name, info.Description)
	})
	// 遍历所有边，写入边定义
	graph.Edges(func(state, destinationState plinko.State, name plinko.Trigger) {
		d.edge(string(state), string(destinationState), string(name))
	})
	d.endGraph()
	return d.err
}

// beginGraph 写入 DOT 图头及样式。
func (d *Dot) beginGraph() {
	d.write([]byte("digraph {\n"))
	d.write([]byte(d.style.graphHeader))
	d.write([]byte(d.style.defaults.graph))
	d.write([]byte(d.style.defaults.node))
	d.write([]byte(d.style.defaults.edge))
}

// endGraph 结束图。
func (d *Dot) endGraph() {
	d.write([]byte("}\n"))
}

// edge 写入一条有向边。
func (d *Dot) edge(a, b, label string) {
	d.write([]byte(fmt.Sprintf(d.style.templates.edge, a, b, label)))
}

// node 写入一个节点，使用 HTML-like 标签来渲染名称和描述。
func (d *Dot) node(name, label, description string) {
	d.write([]byte(fmt.Sprintf(d.style.templates.node, name, label, description)))
}

// DotFileToImg 调用系统 dot 命令将 DOT 文件转换为图片。
func DotFileToImg(from, to, format string) error {
	_, err := exec.Command("sh", "-c", "dot -T"+format+" "+from+" -o "+to).Output()
	return err
}

// dotStylesheet 定义了 DOT 样式。
type dotStylesheet struct {
	graphHeader string
	defaults    dotDefaultStyles
	templates   dotTemplates
}

type dotDefaultStyles struct {
	graph string
	node  string
	edge  string
}

type dotTemplates struct {
	node string // 节点模板，%s 分别是 name, label, description
	edge string // 边模板，%s 分别是 source, destination, label
}

// defaultDotStyle 是默认的 DOT 样式表，使用 fdp 布局，橙色圆角矩形。
var defaultDotStyle = dotStylesheet{
	graphHeader: `layout=fdp;
overlap=false;
sep=1.5;
maxiter=2000;
start=1251;
`,
	defaults: dotDefaultStyles{
		graph: "graph [splines=\"spline\", ranksep=\"2\", nodesep=\"1\"];\n",
		node:  "node [shape=plaintext];\n",
		edge:  "edge [constraint=true, fontname = \"sans-serif\"];\n",
	},
	templates: dotTemplates{
		node: `"%s" [label=<<TABLE STYLE="ROUNDED" BGCOLOR="orange" BORDER="1" CELLSPACING="0" WIDTH="20"><TR><TD BORDER="0">%s</TD></TR><TR><TD BORDER="1" SIDES="t">%s</TD></TR></TABLE>>];` + "\n",
		edge: "\"%s\" -> \"%s\"[label=\"%s\"];\n",
	},
}
