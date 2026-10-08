/*
** Copyright (c) 2026 Oracle and/or its affiliates.
**
** The Universal Permissive License (UPL), Version 1.0
**
** Subject to the condition set forth below, permission is hereby granted to any
** person obtaining a copy of this software, associated documentation and/or data
** (collectively the "Software"), free of charge and under any and all copyright
** rights in the Software, and any and all patent rights owned or freely
** licensable by each licensor hereunder covering either (i) the unmodified
** Software as contributed to or provided by such licensor, or (ii) the Larger
** Works (as defined below), to deal in both
**
** (a) the Software, and
** (b) any piece of software and/or hardware listed in the lrgrwrks.txt file if
** one is included with the Software (each a "Larger Work" to which the Software
** is contributed by such licensors),
**
** without restriction, including without limitation the rights to copy, create
** derivative works of, display, perform, and distribute the Software and make,
** use, sell, offer for sale, import, export, have made, and have sold the
** Software and the Larger Work(s), and to sublicense the foregoing rights on
** either these or other terms.
**
** This license is subject to the following condition:
** The above copyright notice and either this complete permission notice or at
** a minimum a reference to the UPL must be included in all copies or
** substantial portions of the Software.
**
** THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
** IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
** FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
** AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
** LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
** OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
** SOFTWARE.
 */

package ttc

import (
	"context"
	"github.com/oracle/go-oracledb/v26/internal/common"
	driverCommon "github.com/oracle/go-oracledb/v26/internal/driver/common"
	oracleErrors "github.com/oracle/go-oracledb/v26/oracle/errors"
)

// columnPayload holds raw wire bytes or an already constructed value (for example
// driver.Rows), with its optional LOB metadata. Byte payloads still use the type
// decoder; constructed values pass through unchanged. Handlers normalize NULL to nil.
type columnPayload struct {
	data any
	lob  *lobColumnContext
}

// clone detaches wire bytes while preserving identity for constructed values.
// LOB metadata is immutable after unmarshalling and is shared with the copy.
func (p columnPayload) clone() columnPayload {
	if data, ok := p.data.(driverCommon.B1Array); ok {
		p.data = append(driverCommon.B1Array(nil), data...)
	}
	return p
}

// columnUnmarshalContext carries column metadata, position and session character sets.
// Handlers must consume exactly one column, and must not modify row storage.
type columnUnmarshalContext struct {
	column       columnContext
	index        int
	sessCharSet  driverCommon.UB2
	sessNCharSet driverCommon.UB2
}

type columnUnmarshalFunc func(context.Context, driverCommon.Marshaller, columnUnmarshalContext) (columnPayload, error)

func unmarshalCLRColumn(ctx context.Context, mar driverCommon.Marshaller, env columnUnmarshalContext) (columnPayload, error) {
	var payload columnPayload

	colData, length, err := mar.UnmarshalCLRColumnData(ctx)
	if err != nil {
		common.Odl.Warn("Failed to unmarshal column data column", "index", env.index, "error", err)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	common.Odl.Debug("RXD Unmarshal: column data decoded",
		"col", env.index,
		"length", length,
		"data", colData)
	payload = byteColumnPayload(colData)
	return payload, nil
}

func unmarshalClobColumn(ctx context.Context, mar driverCommon.Marshaller, env columnUnmarshalContext) (columnPayload, error) {
	var payload columnPayload

	// length
	lob := &lobColumnContext{}
	var err error
	if lob.LobLength, err = mar.UnmarshalUB4(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read LOB length",
			"error", err, "stage", "lob-length", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	if lob.LobLength == 0 {
		payload.data = nil
		payload.lob = lob
		return payload, nil
	}

	// prefetched: always for V1
	// ------------------------------------------
	// prefetched length
	if lob.PrefetchLength, err = mar.UnmarshalUB8(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read prefetch length",
			"error", err, "stage", "prefetch-length", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	// prefetched chunk size
	if lob.PrefetchChunkSize, err = mar.UnmarshalUB4(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read prefetch chunk size",
			"error", err, "stage", "prefetch-chunk-size", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	var dbVary bool
	var ub1 driverCommon.UB1
	if ub1, err = mar.UnmarshalUB1(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read dbVary flag",
			"error", err, "stage", "db-vary", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	dbVary = byte(ub1) == 0x1
	if dbVary {
		// characterset
		if lob.CharsetID, err = mar.UnmarshalUB2(ctx); err != nil {
			common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read charset ID",
				"error", err, "stage", "charset-id", "index", env.index)
			return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
		}
	}

	// the form of use
	if lob.CharsetForm, err = mar.UnmarshalUB1(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read charset form",
			"error", err, "stage", "charset-form", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}

	// If the character set ID was not returned by the server, use the session
	// character set
	if lob.CharsetID == 0 {
		if lob.CharsetForm == 2 {
			lob.CharsetID = env.sessNCharSet
		} else {
			lob.CharsetID = env.sessCharSet
		}
	}

	colData, length, err := mar.UnmarshalCLRColumnData(ctx)
	if err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read prefetched column data",
			"error", err, "stage", "prefetched-data", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	common.Odl.Debug("RXD Unmarshal: column data decoded",
		"col", env.index,
		"length", length,
		"data", colData)
	payload = byteColumnPayload(colData)
	// ------------------------------------------

	// locator
	if lob.LobLocator, _, err = mar.UnmarshalCLRColumnData(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalClobColumn: failed to read LOB locator",
			"error", err, "stage", "lob-locator", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}

	payload.lob = lob

	return payload, nil

}

func unmarshalBlobColumn(ctx context.Context, mar driverCommon.Marshaller, env columnUnmarshalContext) (columnPayload, error) {
	var payload columnPayload

	// length
	lob := &lobColumnContext{}
	var err error
	if lob.LobLength, err = mar.UnmarshalUB4(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read LOB length",
			"error", err, "stage", "lob-length", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	if lob.LobLength == 0 {
		payload.data = nil
		payload.lob = lob
		return payload, nil
	}

	// prefetched: always for V1
	// ------------------------------------------
	// prefetched length
	if lob.PrefetchLength, err = mar.UnmarshalUB8(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read prefetch length",
			"error", err, "stage", "prefetch-length", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	// prefetched chunk size
	if lob.PrefetchChunkSize, err = mar.UnmarshalUB4(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read prefetch chunk size",
			"error", err, "stage", "prefetch-chunk-size", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}

	colData, length, err := mar.UnmarshalCLRColumnData(ctx)
	if err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read prefetched column data",
			"error", err, "stage", "prefetched-data", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	common.Odl.Debug("RXD Unmarshal: column data decoded",
		"col", env.index,
		"length", length,
		"data", colData)
	payload = byteColumnPayload(colData)
	// ------------------------------------------

	// locator
	if lob.LobLocator, _, err = mar.UnmarshalCLRColumnData(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read LOB locator",
			"error", err, "stage", "lob-locator", "index", env.index)
		return columnPayload{}, common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}

	payload.lob = lob

	return payload, nil

}

func unmarshalJSONIndicator(ctx context.Context, mar driverCommon.Marshaller, env columnUnmarshalContext) error {
	var err error
	if _, err = mar.UnmarshalSB2(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read JSON indicator 1",
			"error", err, "stage", "json-indicator-1", "index", env.index)
		return common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	if _, err = mar.UnmarshalUB2(ctx); err != nil {
		common.Odl.Error("tTIrxd._unmarshalBlobColumn: failed to read JSON indicator 2",
			"error", err, "stage", "json-indicator-2", "index", env.index)
		return common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[TTIRXD])
	}
	return nil
}

func unmarshalJSONColumn(ctx context.Context, mar driverCommon.Marshaller, env columnUnmarshalContext) (columnPayload, error) {
	payload, err := unmarshalBlobColumn(ctx, mar, env)
	if err != nil {
		return columnPayload{}, err
	}
	if err = unmarshalJSONIndicator(ctx, mar, env); err != nil {
		return columnPayload{}, err
	}
	return payload, nil
}

func byteColumnPayload(data driverCommon.B1Array) columnPayload {
	if len(data) == 0 {
		return columnPayload{}
	}
	return columnPayload{data: data}
}
