# Records and struct tags

- [How a struct maps to columns](#how-a-struct-maps-to-columns)
- [Tag reference](#tag-reference)
- [Auto fields](#auto-fields)
- [Omitting nil and empty values](#omitting-nil-and-empty-values)
- [Embedded structs](#embedded-structs)
- [Column lists: RecordColumns and InsertColumns](#column-lists-recordcolumns-and-insertcolumns)
- [The field package](#the-field-package)
- [Errors](#errors)

`Insert(...).Rows(records...)` and `Update(...).SetRecord(rec, ...)` read columns and values from
structs. `gohan` does not scan query results into structs; use your driver or a scanning library
for that.

## How a struct maps to columns

Every exported field is a column, in field order. Unexported fields are ignored. The column name
comes from the `db` tag, then the `ch` tag; **without a tag it is the field name in lower case**
(not snake case):

```go
type Account struct {
	ID        int64  `db:"id"`
	OwnerName string `db:"owner_name"`
	CreatedAt string // no tag
	internal  string // unexported: ignored
}

cols, err := gohan.RecordColumns(reflect.TypeOf(Account{}))
fmt.Println(cols, err)
// Output:
// [id owner_name createdat] <nil>
```

A pointer field binds the value it points to, or `nil` (sent as `NULL`) when it is nil.

## Tag reference

| Tag | Effect |
|---|---|
| `db:"name"` | column name; `db:"-"` excludes the field |
| `ch:"name"` | same as `db`, used when there is no `db` tag |
| `db:"name,auto"` | column name, and marks the field as [auto](#auto-fields) |
| `auto:"true"` | marks the field as auto |
| `goqu:"skipinsert"`, `goqu:"skipupdate"` | mark the field as auto (both: see below) |
| `goqu:"omitnil"` | skip the field when it is a nil pointer |
| `goqu:"omitempty"` | skip the field when it holds its type's zero value |
| `grid:"sort"`, `grid:"filter"`, `grid:"search"` | set `Sortable`/`Filterable`/`Searchable` in the field metadata; no effect on SQL |
| `grid:"auto"` | marks the field as auto |
| `alias:"x"`, `json:"x"`, `xml:"x"` | set the metadata `Alias` (first found in that order); no effect on SQL |

Tag values are comma-separated. `omitnil` and `omitempty` only work in the `goqu` tag: in
`db:"x,omitempty"` the option is ignored and the field is still written. Other unrecognized values
in the `db`, `ch`, `grid` and `goqu` tags are kept in the metadata's `DbOptions` and otherwise
ignored.

## Auto fields

An auto field holds a value the database generates, such as a serial id or a default timestamp. It
is skipped by `Rows` (always) and by `SetRecord` (unless `WithAutoFields()` is passed). It is
still listed by `RecordColumns`.

`goqu:"skipinsert"` and `goqu:"skipupdate"` both mark a field as auto, so **either one skips the
field on insert and on update**. This differs from goqu, where each skips only one statement:

```go
type Post struct {
	ID        int64  `db:"id" auto:"true"`
	Title     string `db:"title"`
	CreatedAt string `db:"created_at" goqu:"skipupdate"`
}

cols, err := gohan.InsertColumns(Post{Title: "hello", CreatedAt: "2024-06-01"})
fmt.Println(cols, err)
// Output:
// [title] <nil>
```

## Omitting nil and empty values

`goqu:"omitnil"` skips a nil pointer field and `goqu:"omitempty"` skips a zero value, on both
insert and update. For `Rows`, the first record decides which fields are present, and every other
record must agree (`ErrInconsistentOmit`). See
[INSERT and upsert](insert-and-upsert.md#from-structs-rows).

```go
type Profile struct {
	UserID int64   `db:"user_id"`
	Bio    string  `db:"bio" goqu:"omitempty"`
	Avatar *string `db:"avatar" goqu:"omitnil"`
}

sql, args, err := gohan.Insert("profiles").Rows(Profile{UserID: 1}).Build(gohan.Postgres())
// sql: INSERT INTO "profiles" ("user_id") VALUES ($1)
// args: [1]
```

`SkipZeroValues()` does the same as `omitempty` for every field of one `SetRecord` call.

## Embedded structs

The fields of an embedded (anonymous) exported struct are flattened into the parent, in place:

```go
type Timestamps struct {
	CreatedAt time.Time `db:"created_at" auto:"true"`
	UpdatedAt time.Time `db:"updated_at"`
}
type Invoice struct {
	ID    int64  `db:"id" auto:"true"`
	Label string `db:"label"`
	Timestamps
}

cols, err := gohan.RecordColumns(reflect.TypeOf(Invoice{}))
fmt.Println(cols, err)
// Output:
// [id label created_at updated_at] <nil>
```

`time.Time` is a struct but is mapped as a single column, because it is registered as a
[reserved type](#reserved-types).

Shapes that could map a value to the wrong column are refused with `ErrRecordShape` rather than
guessed at:

- an embedded pointer to a struct (`*Timestamps`);
- an unexported embedded struct;
- an embedded struct with a `db` or `ch` tag;
- two mapped fields with the same Go name, including an outer field that shadows an embedded one.

Two fields with the same column name are refused too. The error is currently `ErrRecordShape` in
most layouts and `ErrDuplicateColumn` when the duplicate comes from an embedded struct that follows
the field, so check for both.

```go
type Base struct {
	ID int64 `db:"id"`
}
type WithPointer struct {
	*Base
	Name string `db:"name"`
}
type Duplicate struct {
	Base
	OtherID int64 `db:"id"`
}

_, err := gohan.RecordColumns(reflect.TypeOf(WithPointer{}))
fmt.Println(errors.Is(err, gohan.ErrRecordShape), err)

_, err = gohan.RecordColumns(reflect.TypeOf(Duplicate{}))
fmt.Println(errors.Is(err, gohan.ErrRecordShape) || errors.Is(err, gohan.ErrDuplicateColumn), err)
// Output:
// true gohan: record type has a field shape gohan cannot map safely: embedded pointer field "Base"
// true gohan: record type has a field shape gohan cannot map safely: duplicate field name for field id
```

## Column lists: RecordColumns and InsertColumns

`RecordColumns(reflect.Type)` returns every column a struct type maps, auto fields included, in
field order. It is handy for a select list that matches the struct:

```go
type User struct {
	ID       int64  `db:"id" auto:"true"`
	Name     string `db:"name"`
	Email    string `db:"email"`
	Password string `db:"-"`
}

cols, _ := gohan.RecordColumns(reflect.TypeOf(User{}))
selectList := make([]any, len(cols))
for i, c := range cols {
	selectList[i] = c
}

sql, args, err := gohan.Select(selectList...).From("users").Build(gohan.Postgres())
// sql: SELECT "id", "name", "email" FROM "users"
// args: []
```

`InsertColumns(record)` returns the columns an insert of that record would write: auto fields,
nil `omitnil` pointers and zero `omitempty` values are left out.

Both apply the same shape checks as `Rows` and `SetRecord`.

## The field package

`github.com/oddbit-project/gohan/field` holds the struct metadata that `gohan` uses.
`field.GetStructMeta(reflect.Type)` returns one `field.Metadata` per mapped field: `Name`,
`DbName`, `Alias`, `Auto`, `OmitNil`, `OmitEmpty`, `Sortable`, `Filterable`, `Searchable`,
`TypeName`, `Type` and `DbOptions`. Results are cached per type.

```go
type Product struct {
	ID    int64   `db:"id" auto:"true"`
	Name  string  `db:"name" json:"title" grid:"sort,search"`
	Price float64 `db:"price" grid:"sort,filter"`
}

meta, err := field.GetStructMeta(reflect.TypeOf(Product{}))
if err != nil {
	panic(err)
}
for _, m := range meta {
	fmt.Println(m.Name, m.DbName, m.Alias, m.Sortable, m.Filterable, m.Searchable)
}
// Output:
// ID id ID false false false
// Name name title true false true
// Price price Price true true false
```

The `grid` flags are metadata only. A listing API can use them as an allowlist for user-chosen
sort and filter columns (see [Identifiers from request
data](raw-and-escape-hatches.md#identifiers-from-request-data)).

`field.GetStructMeta` itself does not reject the unsafe shapes listed under
[Embedded structs](#embedded-structs); `gohan`'s statement builders and `RecordColumns` do.

### Reserved types

A struct-typed field is normally only a problem when embedded, since embedded structs are
flattened. A *reserved* type is mapped as one column even when embedded. `time.Time` is reserved
by default; register other driver-handled struct types (a custom timestamp or decimal type) with
`field.AddReservedType`, using the type's `reflect.Type.String()` name:

```go
field.AddReservedType("decimal.Decimal")
fmt.Println(field.IsReservedType("decimal.Decimal"), field.IsReservedType("time.Time"))
// Output:
// true true
```

Register reserved types during start-up, before any struct that uses them is mapped: metadata is
cached per struct type and is not recomputed afterwards.

## Errors

| Error | Cause |
|---|---|
| `ErrInvalidRecord` | the record is nil, a nil pointer, or not a struct |
| `ErrRecordType` | `Rows` records of different struct types |
| `ErrRecordShape` | an embedded pointer, unexported or tagged embedded struct, two mapped fields with the same Go name, or (in most layouts) two fields with the same column name |
| `ErrDuplicateColumn` | a column set twice in an `UPDATE`, or two fields with the same column name when the second comes from an embedded struct |
| `ErrInconsistentOmit` | `Rows` records disagree on which `omitnil`/`omitempty` fields are present |
| `ErrUnknownField` | an `IncludeFields`/`ExcludeFields` name that matches no field |
| `ErrNoColumns` | a record with no insertable field |
