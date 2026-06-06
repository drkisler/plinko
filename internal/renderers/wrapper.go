// plinko/internal/renderers/wrapper.go

/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package renderers

import (
	"io"
)

// writeWrapper 包装一个 io.Writer，记录第一次写入错误，之后忽略写入。
type writeWrapper struct {
	writer io.Writer
	err    error
}

// write 写入数据，如果之前发生过错误则跳过。
func (w *writeWrapper) write(p []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.writer.Write(p)
}
