/*
** Copyright (c) 2026 Oracle and/or its affiliates.
**
** The Universal Permissive License (UPL), Version 1.0
**
** Subject to the condition set forth below, permission is hereby granted to any
** person obtaining a copy of this software and associated documentation files
** (the "Software"), to deal in the Software without restriction, including
** without limitation the rights to use, copy, modify, merge, publish,
** distribute, sublicense, and/or sell copies of the Software.
**
** THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
** IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
** FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 */

// Package datatype provides public values for Oracle database datatypes.
package datatype

import "database/sql/driver"

// RefCursor is a typed OUT-bind holder for an Oracle REF CURSOR.
//
// Pass a pointer to RefCursor as the destination of sql.Out. Rows returns the
// server cursor after execution. A RefCursor may also be used for internal
// typed cursor binds when a database API requires REF CURSOR OUT parameters.
type RefCursor struct {
	rows driver.Rows
}

// Rows returns the cursor result set. It is nil for a NULL cursor.
func (c *RefCursor) Rows() driver.Rows {
	if c == nil {
		return nil
	}
	return c.rows
}

// SetRows associates rows with c. It is intended for driver transport
// implementations that populate a typed REF CURSOR OUT bind.
func (c *RefCursor) SetRows(rows driver.Rows) {
	if c != nil {
		c.rows = rows
	}
}

// Close closes the server cursor, if it was returned.
func (c *RefCursor) Close() error {
	if c == nil || c.rows == nil {
		return nil
	}
	err := c.rows.Close()
	c.rows = nil
	return err
}
