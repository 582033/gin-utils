package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql" // init mysql
	"github.com/582033/gin-utils/apm"
	"github.com/582033/gin-utils/config"
	ctx2 "github.com/582033/gin-utils/ctx"
	"github.com/582033/gin-utils/log"
	"sync"
	"time"
)

type Conf struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	DBName       string `json:"dbname"`
	Host         string `json:"host"`
	Port         int16  `json:"port"`
	Charset      string `json:"charset"`
	Timeout      int    `json:"timeout"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
	Loc          string `json:"loc"`
}

type DBConn struct {
	db *sql.DB
}

func (dbConn *DBConn) Original() *sql.DB {
	return dbConn.db
}

type Tx struct {
	tx *sql.Tx
}

//保存连接对象
var conn = make(map[string]*DBConn)
var once = sync.Once{}

func (v Conf) String() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&readTimeout=%ds&writeTimeout=%ds&timeout=%ds&parseTime=true&loc=%s",
		v.Username, v.Password, v.Host, v.Port, v.DBName, v.Charset, v.Timeout, v.Timeout, v.Timeout, v.Loc,
	)
}

func (v Conf) maxOpenConns() int {
	if v.MaxOpenConns == 0 {
		log.Fatal("MaxOpenConns Must set")
	}
	return v.MaxOpenConns
}

func (v Conf) maxIdleConns() int {
	if v.MaxIdleConns == 0 {
		log.Fatal("MaxIdleConns Must set")
	}
	return v.MaxIdleConns
}

//_initDB db初始化
func _initDB() {
	once.Do(func() {
		dbConf := config.Get("mysql")
		var data map[string]*Conf
		if err := dbConf.Scan(&data); err != nil {
			log.Fatal("Error parsing database configuration file ", err)
			return
		}
		for k, v := range data {
			dbConn, err := NewMysql(v)
			if err != nil {
				log.Fatal("InitDB ERROR ", k, " ", v.String())
				return
			}
			conn[k] = dbConn
		}
		go stats()
	})
}

func stats() {
	for range time.Tick(3 * time.Second) {
		for k, v := range conn {
			stats := v.db.Stats()
			apm.Gauges(k, "MaxIdleClosed").Update(stats.MaxIdleClosed)
			apm.Gauges(k, "MaxLifetimeClosed").Update(stats.MaxLifetimeClosed)
			apm.Gauges(k, "WaitCount").Update(stats.WaitCount)
			apm.Gauges(k, "Idle").Update(int64(stats.Idle))
			apm.Gauges(k, "InUse").Update(int64(stats.InUse))
			apm.Gauges(k, "MaxOpenConnections").Update(int64(stats.MaxOpenConnections))
			apm.Gauges(k, "OpenConnections").Update(int64(stats.OpenConnections))
			apm.Gauges(k, "WaitDuration").Update(int64(stats.WaitDuration / time.Millisecond))
		}
	}
}

//NewMysql 实例化db
func NewMysql(conf *Conf) (dbConn *DBConn, err error) {

	db, err := sql.Open("mysql", conf.String())
	if err != nil {
		log.Errorf("mysql conn Error %s", err.Error())
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		log.Errorf("Could not establish a connection with the database, detail: %s", err.Error())
		return nil, err
	}
	db.SetMaxOpenConns(conf.maxOpenConns())
	db.SetMaxIdleConns(conf.maxIdleConns())
	return &DBConn{db: db}, nil
}

//DB 对外获取db实例
func DB(key string) *DBConn {
	_initDB()
	if db, ok := conn[key]; ok && db != nil {
		return db
	}
	return nil
}

//Default 获取默认数据库实例
func Default() *DBConn {
	return DB("default")
}

func (dbConn *DBConn) Begin() (*Tx, error) {
	tx, err := dbConn.db.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (dbConn *DBConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := dbConn.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

// Deprecated: Use ExecContext
func (dbConn *DBConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	s := time.Now()
	res, err := dbConn.db.Exec(query, args...)
	var affected int64
	if res != nil {
		affected, _ = res.RowsAffected()
	}
	log.Debugf("DB.Exec SQL: %s, args: %+v, affected: %v: %f", query, args, affected, time.Now().Sub(s).Seconds())
	return res, err
}

// Deprecated: Use QueryContext
func (dbConn *DBConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s := time.Now()
	res, err := dbConn.db.Query(query, args...)
	log.Debugf("DB.Query SQL: %s, args: %+v: %f", query, args, time.Now().Sub(s).Seconds())
	return res, err
}

// Deprecated: Use QueryRowContext
func (dbConn *DBConn) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	s := time.Now()
	res := dbConn.db.QueryRow(query, args...)
	log.Debugf("DB.QueryRow SQL: %s, args: %+v: %f", query, args, time.Now().Sub(s).Seconds())
	return res

}

func (dbConn *DBConn) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s := time.Now()
	res, err := dbConn.db.ExecContext(ctx, query, args...)
	var affected int64
	if res != nil {
		affected, _ = res.RowsAffected()
	}
	log.Debugf("[%+v] DB.ExecContext SQL: %s, args: %+v, affected: %v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, affected, time.Now().Sub(s).Seconds())
	return res, err
}

func (dbConn *DBConn) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s := time.Now()
	res, err := dbConn.db.QueryContext(ctx, query, args...)
	log.Debugf("[%+v] DB.QueryContext SQL: %s, args: %+v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, time.Now().Sub(s).Seconds())
	return res, err
}

func (dbConn *DBConn) QueryRowContext(ctx context.Context, query string, args ...interface{}) squirrel.RowScanner {
	s := time.Now()
	res := dbConn.db.QueryRowContext(ctx, query, args...)
	log.Debugf("[%+v] DB.QueryRowContext SQL: %s, args: %+v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, time.Now().Sub(s).Seconds())
	return res
}

func (tx *Tx) Commit() error {
	return tx.tx.Commit()
}

func (tx *Tx) Rollback() error {
	return tx.tx.Rollback()
}

// Deprecated: Use ExecContext
func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	s := time.Now()
	res, err := tx.tx.Exec(query, args...)
	var affected int64
	if res != nil {
		affected, _ = res.RowsAffected()
	}
	log.Debugf("TX.Exec SQL: %s, args: %+v, affected: %v: %f", query, args, affected, time.Now().Sub(s).Seconds())
	return res, err
}

// Deprecated: Use QueryContext
func (tx *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s := time.Now()
	res, err := tx.tx.Query(query, args...)
	log.Debugf("TX.Query SQL: %s, args: %+v: %f", query, args, time.Now().Sub(s).Seconds())
	return res, err
}

// Deprecated: Use QueryRowContext
func (tx *Tx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	s := time.Now()
	res := tx.tx.QueryRow(query, args...)
	log.Debugf("TX.QueryRow SQL: %s, args: %+v: %f", query, args, time.Now().Sub(s).Seconds())
	return res
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s := time.Now()
	res, err := tx.tx.ExecContext(ctx, query, args...)
	var affected int64
	if res != nil {
		affected, _ = res.RowsAffected()
	}
	log.Debugf("[%+v] TX.ExecContext SQL: %s, args: %+v, affected: %v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, affected, time.Now().Sub(s).Seconds())
	return res, err
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s := time.Now()
	res, err := tx.tx.QueryContext(ctx, query, args...)
	log.Debugf("[%+v] TX.QueryContext SQL: %s, args: %+v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, time.Now().Sub(s).Seconds())
	return res, err
}

func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) squirrel.RowScanner {
	s := time.Now()
	res := tx.tx.QueryRowContext(ctx, query, args...)
	log.Debugf("[%+v] TX.QueryRowContext SQL: %s, args: %+v: %f", ctx.Value(ctx2.BaseContextRequestIDKey), query, args, time.Now().Sub(s).Seconds())
	return res
}
