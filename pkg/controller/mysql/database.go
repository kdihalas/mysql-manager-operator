package mysql

import (
  "database/sql"
  "fmt"
  "github.com/go-logr/logr"
)

func handleDatabase(reqLogger logr.Logger, db *sql.DB, name string, characterset string, collate string) error {
  exists, err := databaseExists(reqLogger, db, name)
  if err != nil {
    return err
  }
  if exists {
    err = alterDatabase(reqLogger, db, name, characterset, collate)
    return err
  } else {
    err = createDatabase(reqLogger, db, name, characterset, collate)
    return err
  }

}

func databaseExists(reqLogger logr.Logger,db *sql.DB, name string) (bool, error) {
  reqLogger.Info("Check if database exists")
  sqlStatement :=  fmt.Sprintf("SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = '%s'", name)
  rows, err := db.Query(sqlStatement)
  if err != nil {
    return false, err
  }
  count := 0
  for rows.Next() {
    count += 1
  }

  if count > 0 {
    return true, nil
  }
  return false, nil
}

func createDatabase(reqLogger logr.Logger,db *sql.DB, name string, characterset string, collate string) error {
  reqLogger.Info("Create database")
  sqlStatement := fmt.Sprintf("CREATE DATABASE %s CHARACTER SET %s COLLATE %s;",name, characterset, collate)
  _, err := db.Query(sqlStatement)
  if err != nil{
    return err
  }
  return nil
}

func alterDatabase(reqLogger logr.Logger,db *sql.DB, name string, characterset string, collate string) error {
  reqLogger.Info("Alter database")
  sqlStatement := fmt.Sprintf("ALTER DATABASE %s CHARACTER SET %s COLLATE %s;",name, characterset, collate)
  _, err := db.Query(sqlStatement)
  if err != nil{
    return err
  }
  return nil
}

func dropDatabase(reqLogger logr.Logger,db *sql.DB, name string) error {
  reqLogger.Info("Drop database")
  sqlStatement := fmt.Sprintf("DROP DATABASE %s;",name)
  _, err := db.Query(sqlStatement)
  if err != nil{
    return err
  }
  return nil
}