[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](http://godoc.org/github.com/mier85/gomigration)

# gomigration
Package gomigration provides some basic functionality to run migrations.

# Example for a batch migration
```
package main

func main() {
	migrations := make([]Migration, 0)
	migrations = append(migrations, Migration{
		Name: "initial",
		Up: func(transaction *dbr.Tx) (rE error) {
			_, rE = transaction.Exec(`CREATE TABLE ` + "`word`" + `(
 id INT NOT NULL AUTO_INCREMENT,
 name VARCHAR(255),
 PRIMARY KEY (id)
 );`)
			return
		},
		Down: func(transaction *dbr.Tx) (rE error) {
			_, rE = transaction.Exec("DROP TABLE `word`")
			return
		},
	})
	db, err := sql.Open("mysql", "user:password!@tcp(host:port)/dbname")
	if nil != err {
		panic(err)
	}
	connection := dbr.NewConnection(db, nil)
	mM := NewMigrationManager(connection)
	mM.MigrationRunner(migrations)
}
```
