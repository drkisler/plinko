// plinko/pkg/config/operation/opts.go
/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package operation

import "github.com/drkisler/plinko"

// WithName 返回一个 OperationOption，用于设置操作名称。
func WithName(name string) func(*plinko.OperationConfig) {
	return func(c *plinko.OperationConfig) {
		c.Name = name
	}
}
