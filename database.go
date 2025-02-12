package fcommon

import (
	"database/sql"

	"github.com/fabiokaelin/fcommon/internal/database"
	"github.com/fabiokaelin/ferror"
	"github.com/jmoiron/sqlx"
)

func InitDatabase() ferror.FError {
	ferr := database.InitDatabase()
	if ferr != nil {
		return ferr
	}
	DBConnection = database.DBConnection
	return nil
}

func RunSQL(query string, parameters ...any) (*sql.Rows, ferror.FError) {
	return database.RunSQL(query, parameters...)
}

func RunSQLRow(query string, parameters ...any) (*sql.Row, ferror.FError) {
	return database.RunSQLRow(query, parameters...)
}

var DBConnection *sqlx.DB
