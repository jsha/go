package main

import (
	"database/sql"
	"database/sql/driver"
	"log"
	"sync"

	mysql "github.com/go-sql-driver/mysql"
)

func New(prefix string, underlying driver.Driver) driver.Driver {
	return &prefixedDB{
		prefix:     prefix,
		underlying: underlying,
	}
}

type prefixedDB struct {
	prefix     string
	underlying driver.Driver
}

func (p *prefixedDB) Open(name string) (driver.Conn, error) {
	conn, err := p.underlying.Open(name)
	if err != nil {
		return nil, err
	}
	return &prefixedConn{
		prefix: p.prefix,
		conn:   conn,
	}, nil
}

type prefixedConn struct {
	prefix string
	conn   driver.Conn
}

func (c *prefixedConn) Prepare(query string) (driver.Stmt, error) {
	query = c.prefix + query
	log.Print(query)
	return c.conn.Prepare(query)
}

func (c *prefixedConn) Close() error {
	return c.conn.Close()
}

func (c *prefixedConn) Begin() (driver.Tx, error) {
	return c.conn.Begin()
}

func main() {
	sql.Register("prefixedmysql", New("SET STATEMENT max_statement_time=0.1 FOR ", mysql.MySQLDriver{}))
	//sql.Register("prefixedmysql", &PrefixedDB{"  "})
	db, err := sql.Open("prefixedmysql", "sa@tcp(boulder-mysql:3306)/boulder_sa_integration")
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	/*
		if _, err := db.Exec("SET SESSION max_statement_time=0.1"); err != nil {
			log.Fatal(err)
		}
	*/
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			if _, err := db.Exec("SELECT 1 FROM (SELECT SLEEP(?)) as subselect;", i); err != nil {
				log.Print(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	rows, err := db.Query("SELECT id FROM authz WHERE registrationId = ?", 300)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Fatal(err)
		}
		log.Print(id)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	db.Close()
}
