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

// tTIimplres decodes the implicit result sets returned by
// DBMS_SQL.RETURN_RESULT. The message starts with the number of result sets,
// followed by one DCB/cursor pair for each result set. Its optional nested
// RXH/RXD/BVC stream is emitted only when the client and server negotiated
// implicit-result prefetch.
type tTIimplres struct {
	// TTIIMPLRES includes is an implicit result set so the message includes
	// messages that are received when a query result is received. This message
	// needs to know how to decode these messages, so it has a handle to the
	// message creation function of these messages in order to create an
	// instance and call the UnmarshalFrom method on that instance.
	newRXH        MessageCreationFunc // RXH message creation function
	newRXD        MessageCreationFunc // RXD message creation function
	newBVC        MessageCreationFunc // DCB message creation function
	newTTIIMPLOER MessageCreationFunc // newTTIIMPLOER message creation function
	codecFactory  codecFactory        // the coded factory that will be used to decode the result set columns
	// dcb decodes the negotiated TTC version of implicit-result metadata.
	// Prefetched RXD messages are created from the active shelf factory so they
	// always match the negotiated connection protocol version.
	dcb *tTIdcb
	// // shelf and sessCtx initialize the rows returned for implicit cursors and
	// // nested REF CURSOR values decoded from prefetched RXD data.
	// shelf   *ttiShelf[driverCommon.MessageType]
	// sessCtx *driverCommon.SessionContext
	// prefetch indicates that RXH/RXD/BVC data follows each cursor descriptor.
	prefetch bool
	// rows preserves the server order of implicit cursors.
	rows []*ttcRowsRefCursor
}

type tTIimplresChildMessages struct {
	rxh     *tTIrxh
	rxd     *tTIrxd
	bvc     *tTIbvc
	implRes *tTIimplres
}

// newTTIimplres creates an implicit-result decoder for TTC versions 12-16.
func newTTIimplres() driverCommon.Message[driverCommon.MessageType] {
	return &tTIimplres{
		newRXH:        newTTIrxh,
		newBVC:        newTTIbvc,
		newRXD:        newTTIrxd,
		newTTIIMPLOER: newTTIimplresOer,
		dcb:           newTTIdcb().(*tTIdcb),
	}
}

// newTTIimplres creates an implicit-result decoder for TTC versions 12-16.
func newTTIimplres14() driverCommon.Message[driverCommon.MessageType] {
	return &tTIimplres{
		newRXH:        newTTIrxh,
		newBVC:        newTTIbvc,
		newRXD:        newTTIrxd,
		newTTIIMPLOER: newTTIimplresOer14,
		dcb:           newTTIdcb().(*tTIdcb),
	}
}

// newTTIimplres17 creates an implicit-result decoder for TTC versions 17-19.
func newTTIimplres17() driverCommon.Message[driverCommon.MessageType] {
	return &tTIimplres{
		newRXH:        newTTIrxh,
		newBVC:        newTTIbvc,
		newRXD:        newTTIrxd,
		newTTIIMPLOER: newTTIimplresOer14,
		dcb:           newTTIdcb17().(*tTIdcb)}
}

// newTTIimplres20 creates an implicit-result decoder for TTC versions 20-23.
func newTTIimplres20() driverCommon.Message[driverCommon.MessageType] {
	return &tTIimplres{
		newRXH:        newTTIrxh,
		newBVC:        newTTIbvc,
		newRXD:        newTTIrxd,
		newTTIIMPLOER: newTTIimplresOer14,
		dcb:           newTTIdcb20().(*tTIdcb)}
}

// newTTIimplres24 creates an implicit-result decoder for TTC version 24 and later.
func newTTIimplres24() driverCommon.Message[driverCommon.MessageType] {
	return &tTIimplres{
		newRXH:        newTTIrxh,
		newBVC:        newTTIbvc,
		newRXD:        newTTIrxd,
		newTTIIMPLOER: newTTIimplresOer14,
		dcb:           newTTIdcb24().(*tTIdcb)}

}

// GetMsgCode identifies this decoder as TTIIMPLRES.
func (p *tTIimplres) GetMsgCode() driverCommon.MessageType { return TTIIMPLRES }

// configure records the negotiated implicit-result prefetch mode.
func (p *tTIimplres) configure(prefetch bool) {
	p.prefetch = prefetch
}

func (p *tTIimplres) setCodecFactory(factory codecFactory) {
	p.codecFactory = factory
}

// // SetShelf supplies the TTC shelf used to initialize implicit cursor rows.
// func (p *tTIimplres) SetShelf(shelf *ttiShelf[driverCommon.MessageType]) {
// 	p.shelf = shelf
// }

// // SetSessionContext supplies the session context used to initialize implicit cursor rows.
// func (p *tTIimplres) SetSessionContext(sessCtx *driverCommon.SessionContext) {
// 	p.sessCtx = sessCtx
// }

/*
UnmarshalFrom decodes all implicit cursor descriptors and their optional
first-round-trip prefetched rows.

Description:

  - Reads the result-set count and each DCB/cursor-ID descriptor emitted by
    DBMS_SQL.RETURN_RESULT.
  - Initializes one RefCursor rows object per server cursor using the injected
    shelf and session context.
  - Delegates the optional RXH/BVC/RXD/OER prefetch stream to unmarshalPrefetch
    when the negotiated capability is enabled.
  - Preserves server result-set order in rows for RowsNextResultSet traversal.

Parameters:

- ctx: context governing protocol unmarshalling.
- mar: TTC marshaller positioned at the TTIIMPLRES payload.

Returns:

  - error: a protocol, descriptor, or prefetch-decoding error; nil after all
    implicit cursors have been decoded.

Notes:

  - The version-specific DCB instance is initialized by the TTIIMPLRES factory
    before this method is called. Prefetch RXD and OER instances are allocated
    from the active shelf factory for each incoming message.
*/
func (p *tTIimplres) UnmarshalFrom(ctx context.Context, mar driverCommon.Marshaller) error {
	resultSetCount, err := mar.UnmarshalUB4(ctx)
	if err != nil {
		return common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[p.GetMsgCode()])
	}
	p.rows = make([]*ttcRowsRefCursor, 0, resultSetCount)
	common.Odl.Fine("Decoding implicit result sets", "count", resultSetCount, "prefetch", p.prefetch)
	for range resultSetCount {
		if err = p.dcb.UnmarshalFrom(ctx, mar); err != nil {
			return err
		}
		columns, err := p.dcb.getColumnContexts()
		if err != nil {
			return err
		}
		cursorID, err := mar.UnmarshalUB4(ctx)
		if err != nil {
			return common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[p.GetMsgCode()])
		}
		common.Odl.Fine("Decoded implicit result cursor descriptor", "cursorID", cursorID, "columns", len(columns), "prefetch", p.prefetch)
		rows := newRefCursorRows(driverCommon.SB4(cursorID), columns)
		if p.prefetch {
			if err = p.unmarshalPrefetch(ctx, mar, rows, columns); err != nil {
				return err
			}
		}
		p.rows = append(p.rows, rows)
	}
	return nil
}

/*
unmarshalPrefetch consumes the nested RXH/BVC/RXD stream for one implicit
cursor until its implicit-result OER terminator is received.

Description:

  - Reads and dispatches RXH, BVC, and RXD messages returned for a single
    implicit cursor in the initial round trip.
  - Reuses queryRunState to retain BVC column-carry state, prior row data, LOB
    context, and nested RefCursor values across RXD messages.
  - Finalizes the supplied rows when the implicit-result OER terminator reports
    success or the expected no-data status.

Parameters:

- ctx: context governing protocol unmarshalling.
- mar: TTC marshaller positioned at the first nested prefetch message.
- rows: initialized rows object that receives the prefetched data.
- columns: cursor descriptor metadata used to configure each RXD message.

Returns:

  - error: a nested message, Oracle error, or unexpected-message error; nil when
    the cursor's prefetch stream terminates successfully.

Notes:

  - A non-prefetched implicit cursor keeps its deferred RefCursor fetch function
    and never enters this method.
*/
func (p *tTIimplres) unmarshalPrefetch(ctx context.Context, mar driverCommon.Marshaller, rows *ttcRowsRefCursor, columns []columnContext) error {
	common.Odl.Fine("Decoding implicit result prefetch", "cursorID", rows.cursorID, "columns", len(columns))
	state := &queryRunState{rows: rows.ttcRows}
	for {
		code, err := mar.UnmarshalUB1(ctx)
		if err != nil {
			common.Odl.Fine("Failed to read implicit result prefetch message type", "cursorID", rows.cursorID, "rows", state.rowCount, "error", err)
			return common.NewOracleError(oracleErrors.FailUnmarshal, err, TTCMsgTypeDescription[p.GetMsgCode()])
		}
		common.Odl.Fine("Decoding implicit result prefetch message", "cursorID", rows.cursorID, "messageType", TTCMsgTypeName[driverCommon.MessageType(code)], "rows", state.rowCount)
		switch driverCommon.MessageType(code) {
		case TTIRXH:
			rxh := p.newRXH().(*tTIrxh)
			if err = rxh.UnmarshalFrom(ctx, mar); err != nil {
				return err
			}
			common.Odl.Fine("Decoded implicit result RXH", "cursorID", rows.cursorID, "iteration", rxh.iterationNum, "iterations", rxh.numItersThisTime, "requests", rxh.numRequest)
		case TTIBVC:
			bvc := p.newBVC().(*tTIbvc)
			bvc.SetNumberOfColumns(driverCommon.UB4(len(columns)))
			if err = bvc.UnmarshalFrom(ctx, mar); err != nil {
				return err
			}
			state.handleBVC(bvc)
			common.Odl.Fine("Decoded implicit result BVC", "cursorID", rows.cursorID, "columns", len(columns), "presentColumns", bvc.bvcColSent.Cardinality())
		case TTIRXD:
			common.Odl.Fine("Decoding implicit result RXD", "cursorID", rows.cursorID, "row", state.rowCount, "hasBVC", state.bvcFound, "hasPreviousRow", state.prevRow != nil)
			// Configure the version-specific RXD decoder with the preceding row
			// state so BVC-omitted columns can be carried into this row.
			rxd := p.newRXD().(*tTIrxd)
			rxd.setBvcState(state.bvcColSent, state.bvcFound)
			rxd.setRowCount(state.rowCount)
			rxd.setNumberOfColumns(driverCommon.UB4(len(columns)))
			rxd.setColumnContexts(columns)
			rxd.setPrevRow(state.prevRow)
			rxd.setCodecFactory(p.codecFactory)
			// Decode the wire image before retaining it: REF CURSOR columns may
			// include nested DCB metadata that is consumed by RXD decoding.
			if err = rxd.UnmarshalFrom(ctx, mar); err != nil {
				return err
			}
			state.rowCount++
			// Retain decoded values and make them the source for the next BVC row.
			state.handleRXDRow(rxd)
			common.Odl.Fine("Decoded implicit result RXD", "cursorID", rows.cursorID, "row", state.rowCount-1, "columns", len(rxd.row), "totalRows", len(rows.rowData))
		case TTIIMPLOER, TTIOER:
			msg := p.newTTIIMPLOER()
			oer := msg.(tTIOerIface)
			if err = oer.(driverCommon.Unmarshallable).UnmarshalFrom(ctx, mar); err != nil {
				return err
			}
			oerErr := oer.getError()
			common.Odl.Fine("Decoded implicit result OER", "cursorID", rows.cursorID, "error", oerErr, "rows", state.rowCount)
			if oerErr != nil {
				sqlErr, ok := oerErr.(oracleErrors.SQLError)
				if !ok || sqlErr.ErrorCode() != string(oracleErrors.NoDataFound) {
					return oerErr
				}
			}
			rows.numOfRows = len(rows.rowData)
			rows.executor = nil
			rows.fetched = true
			common.Odl.Fine("Completed implicit result prefetch", "cursorID", rows.cursorID, "rows", rows.numOfRows)
			return nil
		default:
			common.Odl.Error("Unexpected implicit result prefetch message; invalidating connection", "cursorID", rows.cursorID, "messageType", code, "rows", state.rowCount)
			return common.NewOracleError(oracleErrors.InternalError, nil)
		}
	}
}
