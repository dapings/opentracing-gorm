# opentracing gorm

[OpenTracing](http://opentracing.io/) for [GORM](http://gorm.io/)

## Install

```shell
go get -u github.com/dapings/opentracing-gorm
```

## Usage

1. Call `otgorm.AddGORMCallbacks(db)` with an `*gorm.DB` instance.
2. Clone db `db = otgorm.WithContext(ctx, db)` with a span.

## Example

```go
package demo

import (
	"context"
	"testing"
	
	"github.com/jinzhu/gorm"
	otgorm "github.com/dapings/opentracing-gorm"
	"github.com/opentracing/opentracing-go"
)

var db *gorm.DB

func init() {
    db = initDB()
}

func initDB() *gorm.DB {
    liteDB, err := gorm.Open("sqlite3", ":memory:")
    if err != nil {
        panic(err)
    }
    // register callbacks must be called for a root instance of gorm.DB
    otgorm.AddGORMCallbacks(liteDB)
    return liteDB
}

func Handler(ctx context.Context) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "handler")
    defer span.Finish()

    // clone db with proper context
	clonedDB := otgorm.WithContext(ctx, db)

    // sql query
	var p struct{}
	clonedDB.First(&p, 1)
}

func TestDemo((t *testing.T)) {
	Handler(context.Background())
}
```

Call to the `Handler` function would create sql span with table name, sql method and sql statement as a child of handler span.
