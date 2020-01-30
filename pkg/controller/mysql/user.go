package mysql

import (
  "database/sql"
  "fmt"
  "github.com/go-logr/logr"
  "strings"
)

func handleUser(reqLogger logr.Logger, db *sql.DB, database string, user string, password string, host string, privileges []string) error {
  exists, err := userExists(reqLogger, db, user, host)
  if err != nil {
    return err
  }
  if exists {
    err = alterUser(reqLogger, db, database, user, password, host, privileges)
    return err
  } else {
    err = createUser(reqLogger, db, database, user, password, host, privileges)
    return err
  }
}

func checkUserGrants(reqLogger logr.Logger, db *sql.DB, database string, user string, host string, privileges []string) (bool, error) {
  reqLogger.Info("Check user grants")
  privilegesString := strings.Join(privileges[:], ",")

  sqlStatement := fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s';", user, host)
  rows, err := db.Query(sqlStatement)
  if err != nil {
    return false, err
  }
  grants := make([]string, 0)
  for rows.Next() {
    var grant string
    if err := rows.Scan(&grant); err != nil {
      reqLogger.Error(err, "mysql error")
    }
    grants = append(grants, grant)
  }

  for _, grant := range(grants) {
    if grant == fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%s'", privilegesString, database, user, host) {
      return true, nil
    }
  }

  return false, nil
}

func userExists(reqLogger logr.Logger,db *sql.DB, user string, host string) (bool, error) {
  reqLogger.Info("Check if user exists")
  sqlStatement := fmt.Sprintf("SELECT * FROM mysql.user WHERE user = '%s' AND host = '%s'", user, host)
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

func createUser(reqLogger logr.Logger, db *sql.DB, database string, user string, password string, host string, privileges []string) error {
  reqLogger.Info("Creating user")
  createUserStatement := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s';", user, host, password)
  _, err := db.Query(createUserStatement)
  if err != nil {
    return err
  }

  reqLogger.Info("Grant user privileges")
  privilegesString := strings.Join(privileges[:], ",")
  privilegesStatement := fmt.Sprintf("GRANT %s ON %s.* TO '%s'@'%s';", privilegesString, database, user, host)
  _, err = db.Query(privilegesStatement)
  if err != nil {
    return err
  }
  _, err = flushPrivileges(reqLogger, db)
  if err != nil {
    return err
  }
  return nil
}

func alterUser(reqLogger logr.Logger, db *sql.DB, database string, user string, password string, host string, privileges []string) error {
  reqLogger.Info("Alter user")
  alterUserStatement := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s';", user, host, password)
  _, err := db.Query(alterUserStatement)
  if err != nil {
    return err
  }
  if exists, _ := checkUserGrants(reqLogger, db, database, user, host, privileges); !exists {
    reqLogger.Info("Revoke all privileges")
    revokePrivilegesStatement := fmt.Sprintf("REVOKE ALL PRIVILEGES ON %s.* FROM '%s'@'%s'", database, user, host);
    _, err := db.Query(revokePrivilegesStatement)
    if err != nil {
      return err
    }
    privilegesString := strings.Join(privileges[:], ",")
    reqLogger.Info("Grant privileges")
    privilegesStatement := fmt.Sprintf("GRANT %s ON %s.* TO '%s'@'%s';", privilegesString, database, user, host)
    _, err = db.Query(privilegesStatement)
    if err != nil {
      return err
    }
    _, err = flushPrivileges(reqLogger, db)
    if err != nil {
      return err
    }
  }
  return nil
}

func dropUser(reqLogger logr.Logger, db *sql.DB, user string, host string) error {
  reqLogger.Info("Delete user")
  dropUserStatement := fmt.Sprintf("DROP USER '%s'@'%s';", user, host)
  _, err := db.Query(dropUserStatement)
  if err != nil {
    return err
  }
  return nil
}

func flushPrivileges(reqLogger logr.Logger, db *sql.DB) (interface{}, error) {
  reqLogger.Info("Flushing privileges")
  return db.Query("FLUSH PRIVILEGES;")
}