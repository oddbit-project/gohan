# SQLite example

Creates an in-memory SQLite database with `modernc.org/sqlite` (pure Go, no cgo) and runs an
`INSERT` from structs, an upsert (`ON CONFLICT ... DO UPDATE`), an `INSERT ... RETURNING`, a
`SELECT`, an `UPDATE` from a struct, and a `DELETE`. It also shows that a `DELETE` without `WHERE`
is refused.

No setup is needed:

```
cd examples
go run ./sqlite
```

The output starts with:

```
insert:
  INSERT INTO `users` (`name`, `email`, `visits`) VALUES (?, ?, ?), (?, ?, ?)
  args: [alice alice@example.com 1 bob bob@example.com 1]
  -> rows affected: 2
```
