package field_test

import (
	"fmt"
	"reflect"

	"github.com/oddbit-project/gohan/field"
)

// ExampleGetStructMeta reads the column mapping of a struct. Embedded
// structs are flattened, db:"-" excludes a field, and the alias comes from
// the alias, json or xml tag.
func ExampleGetStructMeta() {
	type Audit struct {
		CreatedBy string `db:"created_by"`
	}
	type Product struct {
		ID       int64   `db:"id" auto:"true"`
		Name     string  `db:"name" json:"title" grid:"sort,search"`
		Price    float64 `db:"price" grid:"sort,filter"`
		Discount *int    `db:"discount" goqu:"omitnil"`
		Secret   string  `db:"-"`
		Audit
	}

	meta, err := field.GetStructMeta(reflect.TypeOf(Product{}))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, m := range meta {
		fmt.Printf("%-9s db=%-10s alias=%-9s auto=%-5v omitnil=%-5v sort=%-5v filter=%-5v search=%v\n",
			m.Name, m.DbName, m.Alias, m.Auto, m.OmitNil, m.Sortable, m.Filterable, m.Searchable)
	}
	// Output:
	// ID        db=id         alias=ID        auto=true  omitnil=false sort=false filter=false search=false
	// Name      db=name       alias=title     auto=false omitnil=false sort=true  filter=false search=true
	// Price     db=price      alias=Price     auto=false omitnil=false sort=true  filter=true  search=false
	// Discount  db=discount   alias=Discount  auto=false omitnil=true  sort=false filter=false search=false
	// CreatedBy db=created_by alias=CreatedBy auto=false omitnil=false sort=false filter=false search=false
}

// Timestamp is a struct-based value type that a database driver handles
// itself, so it must be mapped as one column rather than flattened.
type Timestamp struct {
	unix int64
}

// ExampleAddReservedType registers a struct type that must be mapped as a
// single column when it is embedded, like time.Time. Register reserved
// types at start-up, before GetStructMeta first sees a struct that uses
// them: metadata is cached per type.
func ExampleAddReservedType() {
	type Event struct {
		Timestamp
		Name string `db:"name"`
	}

	name := reflect.TypeOf(Timestamp{}).String()
	fmt.Println(name, field.IsReservedType(name))

	field.AddReservedType(name)
	fmt.Println(name, field.IsReservedType(name))

	meta, _ := field.GetStructMeta(reflect.TypeOf(Event{}))
	for _, m := range meta {
		fmt.Println(m.Name, m.DbName, m.TypeName)
	}
	// Output:
	// field_test.Timestamp false
	// field_test.Timestamp true
	// Timestamp timestamp field_test.Timestamp
	// Name name string
}
