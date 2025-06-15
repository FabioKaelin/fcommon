package database

import (
	// "backend/config"

	"database/sql"
	"fmt"
	"time"

	// database driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/fabiokaelin/fcommon/pkg/logger"
	"github.com/fabiokaelin/fcommon/pkg/values"
	"github.com/fabiokaelin/ferror"
)

var DBConnection *sqlx.DB
var connectionString string

func InitDatabase() ferror.FError {
	ferr := updateDBConnection()
	if ferr != nil {
		time.Sleep(10 * time.Second)
		ferr = updateDBConnection()
		if ferr != nil {
			time.Sleep(10 * time.Second)
			ferr = updateDBConnection()
			if ferr != nil {
				return ferr
			}
		}
	}

	startDatabaseHealthCheck(5 * time.Minute)
	return nil
}

// updateDBConnection initializes or updates the database connection
func updateDBConnection() ferror.FError {
	if connectionString == "" {
		connectionString = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", values.V.DatabaseValues.DatabaseUser, values.V.DatabaseValues.DatabasePassword, values.V.DatabaseValues.DatabaseHost, values.V.DatabaseValues.DatabasePort, values.V.DatabaseValues.DatabaseName)
	}
	dbNew, err := sqlx.Open("mysql", connectionString)
	if err != nil {
		logger.Log.Error(err.Error())
		DBConnection = nil
		ferr := ferror.FromError(err)
		ferr.SetLayer("db")
		ferr.SetKind("db connection")
		ferr.SetInternal("error during opening db connection")
		return ferr
	}
	if dbNew == nil {
		DBConnection = nil
		ferr := ferror.New("new db connection is nil")
		ferr.SetLayer("db")
		ferr.SetKind("db connection")
		ferr.SetInternal("error during opening db connection")
		return ferr
	}
	// test if connection is working
	err = dbNew.Ping()
	if err != nil {
		fmt.Println(err)
		DBConnection = nil
		ferr := ferror.New("ping to db failed")
		ferr.SetLayer("db")
		ferr.SetKind("db connection")
		return ferr
	}
	if DBConnection != nil {
		DBConnection.Close()
		DBConnection = nil
	}
	DBConnection = dbNew
	DBConnection.SetMaxOpenConns(100)
	DBConnection.SetMaxIdleConns(30)
	maxLifeTime := time.Minute * 30
	DBConnection.SetConnMaxLifetime(maxLifeTime)
	return nil
}

// startDatabaseHealthCheck startet eine Hintergrundroutine für regelmässige Pings
func startDatabaseHealthCheck(interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if DBConnection == nil {
				logger.Log.Warn("DBConnection is nil, trying to reconnect...")
				_ = updateDBConnection()
				continue
			}

			err := DBConnection.Ping()
			if err != nil {
				logger.Log.Warn("Ping failed, reconnecting... Error: " + err.Error())
				_ = updateDBConnection()
			}
		}
	}()
}

// RunSQL executes a query
func RunSQL(query string, parameters ...any) (*sql.Rows, ferror.FError) {
	if DBConnection != nil {
		// err := DBConnection.Ping()
		// if err != nil {
		// 	logger.Log.Warn("DB Connection lost, reconnecting...")
		// 	ferr := updateDBConnection()
		// 	if ferr != nil {
		// 		return &sql.Rows{}, ferr
		// 	}
		// }
		rows, err := DBConnection.Query(query, parameters...)
		if err != nil {
			ferr := ferror.FromError(err)
			ferr.SetLayer("db")
			ferr.SetKind("db execution")
			ferr.SetInternal("error during executing " + query)
			return &sql.Rows{}, ferr
		}
		return rows, nil
	}
	ferr := ferror.New("no db connection")
	ferr.SetLayer("db")
	ferr.SetKind("db execution")
	return &sql.Rows{}, ferr
}

// RunSQLRow executes a query and returns a row
func RunSQLRow(query string, parameters ...any) (*sql.Row, ferror.FError) {
	if DBConnection != nil {
		err := DBConnection.Ping()
		if err != nil {
			logger.Log.Warn("DB Connection lost, reconnecting...")
			ferr := updateDBConnection()
			if ferr != nil {
				return &sql.Row{}, ferr
			}
		}
		rows := DBConnection.QueryRow(query, parameters...)
		return rows, nil
	}
	ferr := ferror.New("no db connection")
	ferr.SetLayer("db")
	ferr.SetKind("db execution")
	ferr.SetInternal("error during executing " + query)
	return &sql.Row{}, ferr
}
