// Package gomigration provides some basic functionality to run migrations.
package gomigration

import (
	"errors"
	"fmt"
	"time"

	"github.com/gocraft/dbr"
)

type (
	Migrate   func(*dbr.Tx) error
	Migration struct {
		Name     string
		Up, Down Migrate
	}
	MigrationManager struct {
		Connection *dbr.Connection
		tableName  string
	}
)

// NewMigrationManager returns a default MigrationManager and initializes it.
func NewMigrationManager(c *dbr.Connection) MigrationManager {
	mM := MigrationManager{Connection: c, tableName: "dbMigrations"}
	mM.Init()
	return mM
}

// NewMigrationManagerExplicitTableName returns a new MigrationManager with a named migration-meta-data table and initializes it.
func NewMigrationManagerExplicitTableName(c *dbr.Connection, tableName string) MigrationManager {
	mM := MigrationManager{Connection: c, tableName: tableName}
	mM.Init()
	return mM
}

// Init initializes the necessary DbTable for the migrations and panics if not successful.
func (mM MigrationManager) Init() {
	session := mM.Connection.NewSession(nil)
	transaction, err := session.Begin()
	if nil != err {
		panic(err)
	}
	_, err = transaction.Exec("CREATE TABLE IF NOT EXISTS `" + mM.tableName + "` " + `(
				id INT NOT NULL AUTO_INCREMENT,
				name VARCHAR(255),
				execution DATETIME,
				PRIMARY KEY (id)
		)`)
	if nil != err {
		transaction.Rollback()
		panic(err)
	}
	err = transaction.Commit()
	if nil != err {
		transaction.Rollback()
	}
}

// MarkAsExecuted marks that a single Migration was applied.
func (mM MigrationManager) MarkAsExecuted(transaction *dbr.Tx, migration Migration) (rErr error) {
	t := time.Now().Format("2006-01-02 15:04:05")
	_, rErr = transaction.InsertInto(mM.tableName).Pair("name", migration.Name).Pair("execution", t).Exec()
	return
}

// MarkAsNotExecuted deletes the entry of an migration that was previously applied.
func (mM MigrationManager) MarkAsNotExecuted(transaction *dbr.Tx, migration Migration) (rErr error) {
	_, rErr = transaction.DeleteFrom(mM.tableName).Where("name = ?", migration.Name).Exec()
	return
}

// CheckIfExecuted checks if an migration ran before and returns true if yes and otherwise false.
func (mM MigrationManager) CheckIfExecuted(session *dbr.Session, migration Migration) bool {
	amount, _ := session.Select("count(*)").From(mM.tableName).Where("name = ?", migration.Name).ReturnInt64()
	return amount > 0
}

// CheckIfSane checks if the list of migrations has any name twice and stops on first error or returns nil.
func (mM MigrationManager) CheckIfSane(migrations []Migration) error {
	list := make(map[string]bool)
	for _, m := range migrations {
		if _, double := list[m.Name]; double {
			return errors.New(fmt.Sprintf("migrations name must be unique but migration \"%s\" exists at least twice", m.Name))
		}
	}
	return nil
}

// MigrationRunner applies all migrations that have not yet been executed.
func (mM MigrationManager) MigrationRunner(migrations []Migration) {
	mM.CheckIfSane(migrations)
	session := mM.Connection.NewSession(nil)
	for _, migration := range migrations {
		if err := mM.RunSingleMigrationUp(session, migration); nil != err {
			panic(err)
		}
	}
}

// RunSingleMigrationUp applies a single migration if it was not yet executed.
func (mM MigrationManager) RunSingleMigrationUp(session *dbr.Session, migration Migration) error {
	if mM.CheckIfExecuted(session, migration) {
		return nil
	}
	transaction, err := session.Begin()
	if nil != err {
		return err
	}
	err = migration.Up(transaction)
	if nil == err {
		if err := mM.MarkAsExecuted(transaction, migration); nil != err {
			transaction.Rollback()
			return err
		}
		if err2 := transaction.Commit(); nil != err2 {
			transaction.Rollback()
			return err2
		}
	} else {
		transaction.Rollback()
		return err
	}
	return nil
}

// RunSingleMigrationDown undos a migration if it was already applied, otherwise throws an error.
func (mM MigrationManager) RunSingleMigrationDown(session *dbr.Session, migration Migration) error {
	if !mM.CheckIfExecuted(session, migration) {
		return errors.New("migration was not yet executed")
	}
	transaction, err := session.Begin()
	if nil != err {
		return err
	}
	err = migration.Down(transaction)
	if nil == err {
		if err := mM.MarkAsNotExecuted(transaction, migration); nil != err {
			transaction.Rollback()
			return err
		}
		if err2 := transaction.Commit(); nil != err2 {
			transaction.Rollback()
			return err2
		}
	} else {
		transaction.Rollback()
	}
	return nil
}

// A trivial example of running migrations is:
// 		package main
//
// 		func main() {
// 			migrations := make([]Migration, 0)
// 			migrations = append(migrations, Migration{
// 				Name: "initial",
// 				Up: func(transaction *dbr.Tx) (rE error) {
// 					_, rE = transaction.Exec(`CREATE TABLE ` + "`word`" + `(
// 						id INT NOT NULL AUTO_INCREMENT,
// 						name VARCHAR(255),
// 						PRIMARY KEY (id)
// 				 	);`)
// 				return
// 				},
// 				Down: func(transaction *dbr.Tx) (rE error) {
// 					_, rE = transaction.Exec("DROP TABLE `word`")
// 					return
// 				},
// 			})
// 			db, err := sql.Open("mysql", "user:password!@tcp(host:port)/dbname")
// 			if nil != err {
// 				panic(err)
// 			}
// 			connection := dbr.NewConnection(db, nil)
// 			mM := NewMigrationManager(connection)
// 			mM.MigrationRunner(migrations)
//		}
//
// An Example how to undo a single Migration
// 			mM.RunSingleMigrationDown(connection.NewSession(nil), migrations[0])
//
