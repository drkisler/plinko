// plinko/pkg/config/state/opts.go

/**
 * Copyright (c) Shipt.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */
package state

import "github.com/drkisler/plinko"

// WithName 设置状态的显示名称。
func WithName(name string) func(*plinko.StateConfig) {
	return func(c *plinko.StateConfig) {
		c.Name = name
	}
}

// WithDescription 设置状态的描述信息。
func WithDescription(description string) func(*plinko.StateConfig) {
	return func(c *plinko.StateConfig) {
		c.Description = description
	}
}
