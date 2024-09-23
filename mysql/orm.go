package mysql

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/582033/gin-utils/ctx"

	"github.com/ymzuiku/hit"

	sq "github.com/Masterminds/squirrel"
)

//缓反射结构信息
var cache = sync.Map{}

//一个字段信息
type FiledInfo struct {
	Name  string
	Index []int
	Tag   string
	I     int
}

//Table 一张表
type Table interface {
	TableName() string
}

//Row 一条记录
type Row interface {
	Table
}

func toSliceStruct(rows *sql.Rows, t reflect.Type) (interface{}, error) {
	rowNew := reflect.New(t).Interface()
	v := reflect.ValueOf(rowNew)

	fields := getField(rowNew)
	columnNames, _ := rows.Columns()

	var scanDest = make([]interface{}, 0, len(columnNames))
	var addrByColumnName = make(map[string]interface{}, len(fields))

	for k, field := range fields {
		addrByColumnName[k] = v.Elem().FieldByIndex(field.Index).Addr().Interface()
	}

	for _, columnName := range columnNames {
		if v, ok := addrByColumnName[columnName]; ok {
			scanDest = append(scanDest, v)
		} else {
			var tmp interface{}
			scanDest = append(scanDest, &tmp)
		}
	}
	return rowNew, rows.Scan(scanDest...)
}

func getType(tbl interface{}) reflect.Type {
	sv := reflect.ValueOf(tbl)
	// 如果是指针，则获取其所指向的元素
	if sv.Kind() == reflect.Ptr {
		sv = sv.Elem()
	}

	if sv.Kind() == reflect.Slice {
		t := sv.Type().Elem()
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		return t
	}
	return sv.Type()
}

func toStruct(rows *sql.Rows, t reflect.Type) (Row, error) {

	rowNew := reflect.New(t).Interface().(Row)
	v := reflect.ValueOf(rowNew)

	fields := getField(rowNew)
	columnNames, _ := rows.Columns()

	var scanDest = make([]interface{}, 0, len(columnNames))
	var addrByColumnName = make(map[string]interface{}, len(fields))

	for k, field := range fields {
		addrByColumnName[k] = v.Elem().FieldByIndex(field.Index).Addr().Interface()
	}

	for _, columnName := range columnNames {
		if v, ok := addrByColumnName[columnName]; ok {
			scanDest = append(scanDest, v)
		} else {
			var tmp interface{}
			scanDest = append(scanDest, &tmp)
		}
	}
	return rowNew, rows.Scan(scanDest...)
}

//SelectScan
func SelectScan(ctx ctx.BaseContext, db sq.BaseRunner, tbl Table, selectBuilder func(tblName string) sq.SelectBuilder, dest interface{}) (err error) {

	b := selectBuilder(tbl.TableName()).RunWith(db)

	rows, err := b.QueryContext(ctx)
	if err != nil {
		return err
	}

	defer rows.Close()

	if rows != nil {
		v := reflect.ValueOf(dest).Elem()
		var ptr bool
		if v.Type().Elem().Kind() == reflect.Ptr {
			ptr = true
		}

		t := getType(dest)
		for rows.Next() {
			if item, err := toSliceStruct(rows, t); err == nil {
				if ptr {
					v = reflect.Append(v, reflect.ValueOf(item))
				} else {
					v = reflect.Append(v, reflect.ValueOf(item).Elem())
				}
			} else {
				return err
			}
		}
		reflect.ValueOf(dest).Elem().Set(v)
		return nil
	}
	return
}

func SelectOneScan(ctx ctx.BaseContext, runner sq.BaseRunner, tbl Table, selectBuilder func(tblName string) sq.SelectBuilder, dest ...interface{}) error {
	b := selectBuilder(tbl.TableName()).RunWith(runner)
	return b.QueryRowContext(ctx).Scan(dest...)
}

func SelectOne(ctx ctx.BaseContext, runner sq.BaseRunner, tbl Table, selectBuilder func(tblName string) sq.SelectBuilder) (Row, error) {
	rows, err := Select(ctx, runner, tbl, selectBuilder)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}

	return rows[0], nil
}

func Select(ctx ctx.BaseContext, runner sq.BaseRunner, tbl Table, selectBuilder func(tblName string) sq.SelectBuilder) ([]Row, error) {
	b := selectBuilder(tbl.TableName()).RunWith(runner)

	rows, err := b.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows != nil {
		var list []Row
		t := getType(tbl)
		for rows.Next() {
			if item, err := toStruct(rows, t); err == nil {
				list = append(list, item)
			} else {
				return nil, err
			}
		}
		return list, nil
	}
	return nil, fmt.Errorf("mysql select error")
}

//更新
func Update(ctx ctx.BaseContext, runner sq.BaseRunner, tbl Table, updateBuilder func(tblName string) sq.UpdateBuilder) (count int64, err error) {
	b := updateBuilder(tbl.TableName()).RunWith(runner)

	res, err := b.ExecContext(ctx)
	if err != nil {
		return 0, err
	}

	count, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

//删除
func Delete(ctx ctx.BaseContext, runner sq.BaseRunner, tbl Table, deleteBuilder func(tblName string) sq.DeleteBuilder) (count int64, err error) {
	b := deleteBuilder(tbl.TableName()).RunWith(runner)

	res, err := b.ExecContext(ctx)
	if err != nil {
		return 0, err
	}

	count, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

//BulkInsert 批量插入
func BulkInsert(ctx ctx.BaseContext, runner sq.BaseRunner, list ...Row) (count int64, err error) {
	if len(list) == 0 {
		return 0, nil
	}
	per := 100
	batch := len(list) / per
	for i := 0; i <= batch; i++ {
		begin, end := i*per, hit.If(i*per+per > len(list), len(list), i*per+per).(int)
		if begin == end {
			break
		}
		_, c, err := insertMany(ctx, runner, list[begin:end]...)
		if err != nil {
			return count, err
		}
		count = count + c
	}
	return
}

//insertMany 写入
func insertMany(ctx ctx.BaseContext, runner sq.BaseRunner, list ...Row) (lastID, count int64, err error) {
	if len(list) == 0 || len(list) > 100 {
		return 0, 0, fmt.Errorf("batch insertion limit 0-100")
	}
	obj := list[0]
	fields := getField(obj)
	if len(fields) == 0 {
		return 0, 0, fmt.Errorf("add 'orm' tag to the struct")
	}
	fieldList := make([]string, 0, len(fields))
	fieldNames := make([]FiledInfo, 0, len(fields))
	for k, v := range fields {
		fieldList = append(fieldList, k)
		fieldNames = append(fieldNames, v)
	}
	b := sq.Insert(obj.TableName()).Columns(fieldList...)
	for _, info := range list {
		b = b.Values(getValue(info, fieldNames)...)
	}
	b = b.RunWith(runner)

	res, err := b.ExecContext(ctx)
	if err != nil {
		return 0, 0, err
	}
	count, _ = res.RowsAffected()
	lastID, _ = res.LastInsertId()
	return
}

//插入单条
func Insert(ctx ctx.BaseContext, runner sq.BaseRunner, row Row) (lastID int64, err error) {
	lastID, _, err = insertMany(ctx, runner, row)
	return
}

//获取结构中orm信息
func getField(obj interface{}) map[string]FiledInfo {
	t := reflect.TypeOf(obj)
	// 如果是指针，则获取其所指向的元素
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	tbl := t.PkgPath() + "/" + t.Name()
	//log.Debug(tbl)

	if v, ok := cache.Load(tbl); ok {
		return v.(map[string]FiledInfo)
	}
	data := getFieldWithoutCache([]int{}, t, 0)
	cache.Store(tbl, data)
	return data
}

type F []string

func (f F) ToString() string {
	return "`" + strings.Join([]string(f), "`,`") + "`"
}

func Field(obj interface{}) F {
	field := getField(obj)
	list := make(map[int]string)
	for s, info := range field {
		list[info.I] = s
	}
	f := make([]string, 0, len(list))
	for i := 0; i < len(list); i++ {
		f = append(f, list[i])
	}

	return F(f)
}

func GetField(obj interface{}) map[string]FiledInfo {
	return getField(obj)
}

//获取结构中orm信息
func getFieldWithoutCache(index []int, t reflect.Type, n int) map[string]FiledInfo {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	data := make(map[string]FiledInfo)
	for i := 0; i < t.NumField(); i++ {
		item := t.Field(i)
		k := item.Tag.Get("orm")
		if k != "" && k != "-" {
			data[k] = FiledInfo{Name: item.Name, Tag: k, Index: append(index, i), I: n}
			n++
		}
		if !item.Anonymous {
			continue
		}
		if item.Type.Kind() == reflect.Struct {
			m := getFieldWithoutCache(append(index, i), item.Type, n)
			n = n + len(m)
			for k, v := range m {
				data[k] = v
			}
		}
	}
	return data
}

func getValue(obj Table, fs []FiledInfo) []interface{} {
	v := reflect.ValueOf(obj)
	// 如果是指针，则获取其所指向的元素
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	data := make([]interface{}, 0, len(fs))
	for _, f := range fs {
		data = append(data, v.FieldByIndex(f.Index).Interface())
	}
	return data
}

func GetTagWithValue(obj interface{}) map[string]interface{} {
	t := reflect.TypeOf(obj)
	// 如果是指针，则获取其所指向的元素
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	tbl := t.PkgPath() + "/" + t.Name()

	fields := make(map[string]FiledInfo)
	if v, ok := cache.Load(tbl); ok {
		fields = v.(map[string]FiledInfo)
	} else {
		fields = getFieldWithoutCache([]int{}, t, 0)
		cache.Store(tbl, fields)
	}

	v := reflect.ValueOf(obj)
	// 如果是指针，则获取其所指向的元素
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	result := make(map[string]interface{})
	for _, f := range fields {
		result[f.Tag] = v.FieldByIndex(f.Index).Interface()
	}
	return result
}
