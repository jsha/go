package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dbconnect := os.Getenv("DBCONNECT")
	if dbconnect == "" {
		log.Fatal("Must provide $DBCONNECT")
	}
	db, err := sql.Open("mysql", dbconnect)
	if err != nil {
		log.Fatal(err)
	}
	interval := os.Getenv("INTERVAL")
	intervalDur, err := time.ParseDuration(interval)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(intervalDur)
	for {
		go func() {
			_, err = db.Exec(`set transaction isolation level serializable;`)
			if err != nil {
				log.Print(err)
				return
			}
			tx, err := db.Begin()
			if err != nil {
				log.Print(err)
				return
			}
			_, err = tx.Exec(`select * from test.innodb_deadlock_maker where a = 1;`)
			if err != nil {
				log.Print(err)
				return
			}
			_, err = tx.Exec(`update test.innodb_deadlock_maker set a = 1 where a <> 1;`)
			if err != nil {
				log.Print(err)
				return
			}
		}()
		time.Sleep(intervalDur)
	}
}
