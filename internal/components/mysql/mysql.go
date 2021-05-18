package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	log2 "github.com/jiangxw06/go-butler/internal/log"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"sync"

	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"gorm.io/plugin/prometheus"
)

var (
	dbConf dbConfig
	dbOnce sync.Once
)

type (
	dbConfig struct {
		MaxIdle  int
		MaxConn  int
		Connects map[string]string
	}
)

func loadMySQLConfig() {
	if viper.IsSet("db") {
		if err := viper.UnmarshalKey("db", &dbConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		bytes, _ := json.MarshalIndent(dbConf, "", "  ")
		log2.SysLogger().Debugf("db config: \n%v", string(bytes))
	}
}

func MustGetSqlxDB(name string) *sqlx.DB {
	db, err := GetSqlxDB(name)
	if err != nil {
		panic(err)
	}
	return db
}

func GetSqlxDB(name string) (*sqlx.DB, error) {
	dbOnce.Do(loadMySQLConfig)
	dataSource := dbConf.Connects[name]
	db, err := sqlx.Open("mysql", dataSource)
	if err != nil {
		return nil, errors.Errorf("Open database error: %v", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	if dbConf.MaxConn != 0 {
		db.SetMaxOpenConns(dbConf.MaxConn)
	}
	if dbConf.MaxIdle != 0 {
		db.SetMaxIdleConns(dbConf.MaxIdle)
	}
	env.AddShutdownFunc(func() error {
		log2.SysLogger().Infow("db closing...", "name", name)
		err := db.Close()
		if err != nil {
			log2.SysLogger().Errorw("close db err", "err", err, "name", name)
		} else {
			log2.SysLogger().Infow("close db success", "name", name)
		}
		return nil
	})
	return db, nil
}

func MustGetGorm(name string, replicaNames ...string) *gorm.DB {
	db, err := GetGorm(name, replicaNames...)
	if err != nil {
		panic(err)
	}
	return db
}

//replica是从库
func GetGorm(name string, replicaNames ...string) (*gorm.DB, error) {
	dbOnce.Do(loadMySQLConfig)
	if len(replicaNames) > 1 {
		err := errors.New("not supporting multi replicas")
		log2.SysLogger().Error(err)
		return nil, err
	}
	dataSource := dbConf.Connects[name]
	hasReplica := len(replicaNames) > 0
	replicaSource := ""
	if hasReplica {
		replicaSource = dbConf.Connects[replicaNames[0]]
	}

	for i := 0; i < 3; i++ {
		var db *gorm.DB
		var err error
		var sqlDB *sql.DB
		//以后替换为zap日志
		//newLogger := logger.New(
		//	log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		//	logger.Config{
		//		SlowThreshold: time.Second,   // 慢 SQL 阈值
		//		LogLevel:      logger.Silent, // Log level
		//		Colorful:      false,         // 禁用彩色打印
		//	},
		//)

		db, err = gorm.Open(mysql.Open(dataSource), &gorm.Config{
			//Logger: newLogger,
			DisableForeignKeyConstraintWhenMigrating: true,
		})

		if err != nil {
			log2.SysLogger().Errorw("db connect error", "err", err, "name", name, "retry", i)
			continue
		}

		dbresolverConfig := dbresolver.Config{}
		if hasReplica {
			dbresolverConfig.Replicas = []gorm.Dialector{mysql.Open(replicaSource)}
		}

		err = db.Use(dbresolver.Register(dbresolverConfig).
			SetConnMaxIdleTime(time.Minute * 10).
			SetConnMaxLifetime(24 * time.Hour).
			SetMaxIdleConns(dbConf.MaxIdle).
			SetMaxOpenConns(dbConf.MaxConn))
		sqlDB, err = db.DB()

		prometheus.New(prometheus.Config{
			DBName:          name,
			RefreshInterval: 15, //单位：秒

		})

		//todo 可以收集指标到prometheus，详情见 https://gorm.io/zh_CN/docs/prometheus.html

		if err != nil {
			log2.SysLogger().Errorw("db connect error", "err", err, "name", name, "retry", i)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		log2.SysLogger().Infow("db connect success", "name", name)

		//gorm V2 关闭连接的方式是关闭sqlDB
		env.AddShutdownFunc(func() error {
			log2.SysLogger().Infow("db closing...", "name", name)
			err := sqlDB.Close()
			if err != nil {
				log2.SysLogger().Errorw("close db err", "err", err, "name", name)
			} else {
				log2.SysLogger().Infow("close db success", "name", name)
			}
			return nil
		})
		return db, nil
	}

	err := errors.Errorf("cannot connect to db %v", name)
	log2.SysLogger().Error(err)
	return nil, err
}
