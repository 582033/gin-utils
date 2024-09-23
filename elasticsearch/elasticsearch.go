package elasticsearch

import (
	elastic "github.com/olivere/elastic/v7"
	"github.com/582033/gin-utils/config"
	"log"
	"strings"
	"sync"
)

var once = sync.Once{}
var conn = make(map[string]*elastic.Client)

type conf struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

//_initDB 初始化
func _initDB() {
	once.Do(func() {
		esConfig := config.Get("elasticsearch")
		var data map[string]*conf
		if err := esConfig.Scan(&data); err != nil {
			log.Fatal("Error parsing elasticsearch configuration file ", err)
			return
		}
		for k, v := range data {
			esConn, err := elastic.NewClient(
				elastic.SetURL(strings.Split(v.Host, ",")...),
				elastic.SetSniff(false),
				elastic.SetBasicAuth(v.Username, v.Password),
			)
			if err != nil {
				log.Fatalf("error: %s", err.Error())
				return
			}
			conn[k] = esConn
		}
	})
}

//DB 对外获取db实例
func DB(key string) *elastic.Client {
	_initDB()
	if db, ok := conn[key]; ok && db != nil {
		return db
	}
	return nil
}

//Default 获取默认数据库实例
func Default() *elastic.Client {
	return DB("default")
}
