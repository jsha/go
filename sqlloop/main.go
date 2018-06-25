package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	/*
		dbconnect := os.Getenv("DBCONNECT")
		if dbconnect == "" {
			log.Fatal("Must provide $DBCONNECT")
		}
		db, err := sql.Open("mysql", dbconnect)
		if err != nil {
			log.Fatal(err)
		}
		for {
			_, err := db.Query(`SELECT SLEEP(1);`)
			if err != nil {
				log.Print(err)
				time.Sleep(15 * time.Second)
			} else {
				log.Print("ok")
				time.Sleep(15 * time.Second)
			}
		}
	*/
	db, err := sql.Open("mysql", os.Getenv("DBCONNECT"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query(`SET wait_timeout = 3;`)
	if err != nil {
		log.Fatal(err)
	}

	for rows.Next() {
		var x int
		err = rows.Scan(&x)
		if err != nil {
			log.Fatal(err)
		}
		log.Print(x)
	}

	// Wait for 5 seconds. This should be enough to timeout the conn, since `wait_timeout` is 3s
	time.Sleep(5 * time.Second)

	for {
		rows, err := db.Query(`SELECT 42;`)
		if err != nil {
			log.Print(err)
			continue
		}

		for rows.Next() {
			var x int
			err = rows.Scan(&x)
			if err != nil {
				log.Fatal(err)
			}
			log.Print(x)
		}
		time.Sleep(time.Second)
	}
}
