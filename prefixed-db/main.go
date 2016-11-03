package main

import (
	"database/sql"
	"database/sql/driver"
	"log"

	mysql "github.com/go-sql-driver/mysql"
)

type PrefixedDB struct {
	Prefix string
}

func (p *PrefixedDB) Open(name string) (driver.Conn, error) {
	conn, err := mysql.MySQLDriver{}.Open(name)
	if err != nil {
		return nil, err
	}
	return &PrefixedConn{
		Prefix: p.Prefix,
		conn:   conn,
	}, nil
}

type PrefixedConn struct {
	Prefix string
	conn   driver.Conn
}

func (c *PrefixedConn) Prepare(query string) (driver.Stmt, error) {
	query = c.Prefix + query
	log.Print(query)
	s, err := c.conn.Prepare(query)
	log.Print("done ", query)
	return s, err
}

func (c *PrefixedConn) Close() error {
	return c.conn.Close()
}

func (c *PrefixedConn) Begin() (driver.Tx, error) {
	return c.conn.Begin()
}

func main() {
	sql.Register("prefixedmysql", &PrefixedDB{"SET STATEMENT max_statement_time=10 FOR "})
	db, err := sql.Open("prefixedmysql", "sa@tcp(boulder-mysql:3306)/boulder_sa_integration")
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec("SELECT ? FROM (SELECT SLEEP(?)) as subselect;", 1, 1); err != nil {
		log.Fatal(err)
	}
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
