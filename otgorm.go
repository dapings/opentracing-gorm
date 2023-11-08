package otgorm

import (
	"context"
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

const (
	parentSpanGORMKey = "opentracing:parent.span"
	spanGORMKey       = "opentracing:span"
)

// WithContext sets span to gorm settings, returns cloned DB.
func WithContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	return SetSpanToGORM(ctx, db)
}

// SetSpanToGORM sets span to gorm settings, returns cloned DB.
func SetSpanToGORM(ctx context.Context, db *gorm.DB) *gorm.DB {
	if ctx == nil {
		return db
	}
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan == nil {
		return db
	}
	return db.Set(parentSpanGORMKey, parentSpan)
}

// AddGORMCallbacks adds callbacks for tracing, should call SetSpanToGORM to make them work.
func AddGORMCallbacks(db *gorm.DB) {
	callbacks := newCallbacks()
	registerCallbacks(db, callbackCreateName, callbacks)
	registerCallbacks(db, callbackQueryName, callbacks)
	registerCallbacks(db, callbackUpdateName, callbacks)
	registerCallbacks(db, callbackDeleteName, callbacks)
	registerCallbacks(db, callbackRowQueryName, callbacks)
}

var (
	callbackCreateName   = "create"
	callbackQueryName    = "query"
	callbackUpdateName   = "update"
	callbackDeleteName   = "delete"
	callbackRowQueryName = "row_query"
	SQLSpanOpName        = "sql"
)

type callbacks struct{}

func newCallbacks() *callbacks {
	return &callbacks{}
}

func registerCallbacks(db *gorm.DB, name string, c *callbacks) {
	beforeName := fmt.Sprintf("tracing:%v_before", name)
	afterName := fmt.Sprintf("tracing:%v_after", name)
	gormCallbackName := fmt.Sprintf("gorm:%v", name)

	// gorm does some magic things, if pass CallbackProcessor here - nothing works.
	switch name {
	case callbackCreateName:
		db.Callback().Create().Before(gormCallbackName).Register(beforeName, c.beforeCreate)
		db.Callback().Create().After(gormCallbackName).Register(afterName, c.afterCreate)
	case callbackQueryName:
		db.Callback().Query().Before(gormCallbackName).Register(beforeName, c.beforeQuery)
		db.Callback().Query().After(gormCallbackName).Register(afterName, c.afterQuery)
	case callbackUpdateName:
		db.Callback().Update().Before(gormCallbackName).Register(beforeName, c.beforeUpdate)
		db.Callback().Update().After(gormCallbackName).Register(afterName, c.afterUpdate)
	case callbackDeleteName:
		db.Callback().Delete().Before(gormCallbackName).Register(beforeName, c.beforeDelete)
		db.Callback().Delete().After(gormCallbackName).Register(afterName, c.afterDelete)
	case callbackRowQueryName:
		db.Callback().RowQuery().Before(gormCallbackName).Register(beforeName, c.beforeRowQuery)
		db.Callback().RowQuery().After(gormCallbackName).Register(afterName, c.afterRowQuery)
	}
}

func (c *callbacks) before(scope *gorm.Scope) {
	val, ok := scope.Get(parentSpanGORMKey)
	if !ok {
		return
	}
	parentSpan := val.(opentracing.Span)
	tr := parentSpan.Tracer()
	sp := tr.StartSpan(SQLSpanOpName, opentracing.ChildOf(parentSpan.Context()))
	ext.DBType.Set(sp, SQLSpanOpName)
	scope.Set(spanGORMKey, sp)
}

func (c *callbacks) after(scope *gorm.Scope, operation string) {
	val, ok := scope.Get(spanGORMKey)
	if !ok {
		return
	}
	sp := val.(opentracing.Span)
	if operation == "" {
		operation = strings.ToUpper(strings.Split(scope.SQL, " ")[0])
	}
	ext.Error.Set(sp, scope.HasError())
	ext.DBStatement.Set(sp, scope.SQL)
	sp.SetTag("db.table", scope.TableName())
	sp.SetTag("db.method", operation)
	sp.SetTag("db.err", scope.HasError())
	sp.SetTag("db.count", scope.DB().RowsAffected)
	sp.Finish()
}

func (c *callbacks) beforeCreate(scope *gorm.Scope)   { c.before(scope) }
func (c *callbacks) afterCreate(scope *gorm.Scope)    { c.after(scope, "INSERT") }
func (c *callbacks) beforeQuery(scope *gorm.Scope)    { c.before(scope) }
func (c *callbacks) afterQuery(scope *gorm.Scope)     { c.after(scope, "SELECT") }
func (c *callbacks) beforeUpdate(scope *gorm.Scope)   { c.before(scope) }
func (c *callbacks) afterUpdate(scope *gorm.Scope)    { c.after(scope, "UPDATE") }
func (c *callbacks) beforeDelete(scope *gorm.Scope)   { c.before(scope) }
func (c *callbacks) afterDelete(scope *gorm.Scope)    { c.after(scope, "DELETE") }
func (c *callbacks) beforeRowQuery(scope *gorm.Scope) { c.before(scope) }
func (c *callbacks) afterRowQuery(scope *gorm.Scope)  { c.after(scope, "") }
