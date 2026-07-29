package config

import (
	"database/sql"
	"fmt"
	"inkdrop/model"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

var db *bun.DB

func ConnectDatabase() {
	dsn := databaseDSN()
	fmt.Println("Database DSN:", dsn)
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		panic(err)
	}
	if err = sqldb.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	fmt.Println("Database connected.")
	db = bun.NewDB(sqldb, sqlitedialect.New())

	if err := model.ModelUser(db); err != nil {
		log.Printf("WARNING: model.ModelUser: %v", err)
	}
	if err := model.ModelDrop(db); err != nil {
		log.Printf("WARNING: model.ModelDrop: %v", err)
	}
	if err := model.ModelFile(db); err != nil {
		log.Printf("WARNING: model.ModelFile: %v", err)
	}
	if err := model.ModelActivity(db); err != nil {
		log.Printf("WARNING: model.ModelActivity: %v", err)
	}
}

func GetDB() *bun.DB {
	return db
}

func databaseDSN() string {
	rootDir := strings.TrimSpace(os.Getenv("FTR_ROOT_DIR"))
	if rootDir == "" {
		if wd, err := os.Getwd(); err == nil {
			rootDir = wd
		} else {
			rootDir = "/srv/ftr"
		}
	}
	path := filepath.Join(filepath.Clean(rootDir), "database.db")
	abs := filepath.ToSlash(path)
	return "file:" + abs + "?cache=shared&mode=rwc"
}