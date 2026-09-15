# REF CURSOR example

This example demonstrates both supported cursor-return patterns:

- a REF CURSOR returned through a PL/SQL `sql.Out` bind and read with
  `database/sql/driver.Rows`;
- implicit result cursors returned through `DBMS_SQL.RETURN_RESULT` and read
  with `database/sql.Rows.NextResultSet`.

## Prerequisites

- An Oracle Database reachable from this machine.
- Go 1.26 or later.
- A connection string with database credentials.

## Run the example

Set `ORACLE_DSN` to a driver connection string, then run the example from the
repository root:

```bash
export ORACLE_DSN="user/password@localhost:1521/freepdb1"
go run ./examples/datatype/refcursor
```

Expected output is similar to:

```text
Columns: [ID LABEL]
Row: [1 first row]
Row: [2 second row]
Implicit result cursors
Result set 1, columns: [N]
Row: [11]
Result set 2, columns: [N]
Row: [22]
```

## How it works

1. The PL/SQL block opens a server cursor for its OUT bind using `OPEN :1 FOR`.
2. The Go program passes `sql.Out{Dest: &rows}`, where `rows` is a
   `driver.Rows` value.
3. After `ExecContext` completes, the driver assigns the returned server cursor
   to `rows`.
4. The program reads the cursor with `Columns` and `Next`, and closes it with
   `Close` when finished. Closing the rows queues the server-cursor close for a
   subsequent database round trip.

Use one `driver.Rows` destination for each REF CURSOR OUT bind. Always close
each returned cursor, including cursors that are not fully consumed.

## Implicit result cursors

The example also opens two local cursors and exposes them with
`DBMS_SQL.RETURN_RESULT`. No OUT bind is required. `QueryContext` returns a
standard `*sql.Rows`; consume the current result set with `Next` and `Scan`,
then call `NextResultSet` to advance to the next implicit cursor. Close the
outer `*sql.Rows` when finished to release all remaining implicit cursors.
