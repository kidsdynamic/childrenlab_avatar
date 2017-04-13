package database

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DatabaseInfo struct {
	Name     string
	User     string
	Password string
	IP       string
}

var Database DatabaseInfo

func NewDatabase() *sqlx.DB {
	db, err := sqlx.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=true",
		Database.User, Database.Password, Database.IP, Database.Name))

	if err != nil {
		panic(err)
	}

	return db
}
