package otgorm_test

import (
	"context"
	"testing"

	otgorm "github.com/dapings/opentracing-gorm"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

var (
	tracer           *mocktracer.MockTracer
	db               *gorm.DB
	spanSecondOpName = "handler"
)

func init() {
	db = initDB()
	tracer = mocktracer.New()
	opentracing.SetGlobalTracer(tracer)
}

func initDB() *gorm.DB {
	db, err := gorm.Open("sqlite3", ":memory")
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(&Product{})
	db.Create(&Product{Code: "L1108"})
	otgorm.AddGORMCallbacks(db)
	return db
}

type Product struct {
	gorm.Model
	Code string
}

func Handler(ctx context.Context) {
	span, ctx := opentracing.StartSpanFromContext(ctx, spanSecondOpName)
	defer span.Finish()

	db := otgorm.WithContext(ctx, db)
	var product Product
	db.First(&product, 1)
}

func TestPool(t *testing.T) {
	Handler(context.Background())
	spans := tracer.FinishedSpans()
	if len(spans) != 2 {
		t.Fatalf("should be 2 finished spans, but there are %d: %v", len(spans), spans)
	}

	sqlSpan := spans[0]
	if sqlSpan.OperationName != otgorm.SQLSpanOpName {
		t.Errorf("first span operation name should be %s, but it's '%s'", otgorm.SQLSpanOpName, sqlSpan.OperationName)
	}

	expectedTags := map[string]interface{}{
		"error":        false,
		"db.table":     "products",
		"db.method":    "SELECT",
		"db.type":      "sql",
		"db.statement": `SELECT * FROM "products"  WHERE "products"."deleted_at" IS NULL AND (("products"."id" = 1)) ORDER BY "products"."id" ASC LIMIT 1`,
		"db.err":       false,
		"db.count":     int64(1),
	}

	sqlTags := sqlSpan.Tags()
	if len(sqlTags) != len(expectedTags) {
		t.Errorf("sql span should have %d tags, but it has %d", len(expectedTags), len(sqlTags))
	}

	for name, expected := range expectedTags {
		val, ok := sqlTags[name]
		if !ok {
			t.Errorf("sql span doesn't have tag '%s'", name)
		}
		if val != expected {
			t.Errorf("sql span tag '%s' should have value '%s', but it has '%s'", name, expected, val)
		}
	}
	if spans[1].OperationName != spanSecondOpName {
		t.Errorf("second span operation name should %s, bug it/s '%s'", "handler", spans[1].OperationName)
	}
}
