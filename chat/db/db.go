package db

import (
	"context"
	"database/sql"
	dbP "database/sql"
	rice "github.com/GeertJohan/go.rice"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	. "nkonev.name/chat/logger"
	"time"
)

// https://medium.com/@benbjohnson/structuring-applications-in-go-3b04be4ff091
type DB struct {
	*sql.DB
}

type MigrationDB struct {
	*sql.DB
}

type Tx struct {
	*sql.Tx
}

// enumerates common tx and non-tx operations
type CommonOperations interface {
	Query(query string, args ...interface{}) (*dbP.Rows, error)
	QueryRow(query string, args ...interface{}) *dbP.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	GetParticipantIds(chatId int64) ([]int64, error)
	IsAdmin(userId int64, chatId int64) (bool, error)
	GetChat(participantId, chatId int64) (*Chat, error)
	GetChatWithParticipants(behalfParticipantId, chatId int64) (*ChatWithParticipants, error)
	GetMessage(chatId int64, userId int64, messageId int64) (*Message, error)
	GetUnreadMessages(chatId int64, userId int64) (int64, error)
}

func (dbR *DB) Query(query string, args ...interface{}) (*dbP.Rows, error) {
	return dbR.DB.Query(query, args...)
}

func (txR *Tx) Query(query string, args ...interface{}) (*dbP.Rows, error) {
	return txR.Tx.Query(query, args...)
}

func (dbR *DB) QueryRow(query string, args ...interface{}) *dbP.Row {
	return dbR.DB.QueryRow(query, args...)
}

func (txR *Tx) QueryRow(query string, args ...interface{}) *dbP.Row {
	return txR.Tx.QueryRow(query, args...)
}

func (dbR *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return dbR.DB.Exec(query, args...)
}

func (txR *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return txR.Tx.Exec(query, args...)
}

const postgresDriverString = "pgx"

// Open returns a DB reference for a data source.
func Open(conninfo string, maxOpen int, maxIdle int, maxLifetime time.Duration) (*DB, error) {
	if db, err := sql.Open(postgresDriverString, conninfo); err != nil {
		return nil, err
	} else {
		db.SetConnMaxLifetime(maxLifetime)
		db.SetMaxIdleConns(maxIdle)
		db.SetMaxOpenConns(maxOpen)
		return &DB{db}, nil
	}
}

// Begin starts an returns a new transaction.
func (db *DB) Begin() (*Tx, error) {
	if tx, err := db.DB.Begin(); err != nil {
		return nil, err
	} else {
		return &Tx{tx}, nil
	}
}

func (tx *Tx) SafeRollback() {
	if err0 := tx.Rollback(); err0 != nil {
		Logger.Errorf("Error during rollback tx %v", err0)
	}
}

func migrateInternal(db *sql.DB) {
	const migrations = "migrations"
	box := rice.MustFindBox(migrations).HTTPBox()
	src, err := httpfs.New(box, ".")
	if err != nil {
		Logger.Fatal(err)
	}

	d, err := time.ParseDuration("15m")
	if err != nil {
		Logger.Fatal(err)
	}

	pgInstance, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable:  "go_migrate",
		StatementTimeout: d,
	})
	if err != nil {
		Logger.Fatal(err)
	}

	m, err := migrate.NewWithInstance("httpfs", src, "", pgInstance)
	if err != nil {
		Logger.Fatal(err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err.Error() != "no change" {
		Logger.Fatal(err)
	}
}

func (db *MigrationDB) Migrate() {
	Logger.Infof("Starting migration")
	migrateInternal(db.DB)
	Logger.Infof("Migration successful completed")
}

func ConfigureDb(lc fx.Lifecycle) (DB, error) {
	dbConnectionString := viper.GetString("postgresql.url")
	maxOpen := viper.GetInt("postgresql.maxOpenConnections")
	maxIdle := viper.GetInt("postgresql.maxIdleConnections")
	maxLifeTime := viper.GetDuration("postgresql.maxLifetime")
	dbInstance, err := Open(dbConnectionString, maxOpen, maxIdle, maxLifeTime)

	if lc != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				Logger.Infof("Stopping db connection")
				return dbInstance.Close()
			},
		})
	}

	return *dbInstance, err
}

func ConfigureMigrationDb(lc fx.Lifecycle) (DB, error) {
	dbConnectionString := viper.GetString("postgresql.url")
	maxOpen := viper.GetInt("postgresql.maxOpenConnections")
	maxIdle := viper.GetInt("postgresql.maxIdleConnections")
	maxLifeTime := viper.GetDuration("postgresql.maxLifetime")
	dbInstance, err := Open(dbConnectionString, maxOpen, maxIdle, maxLifeTime)

	if lc != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				Logger.Infof("Stopping db connection")
				return dbInstance.Close()
			},
		})
	}

	return *dbInstance, err
}

func (db *DB) RecreateDb() {
	_, err := db.Exec(`
	DROP SCHEMA IF EXISTS public CASCADE;
	CREATE SCHEMA IF NOT EXISTS public;
    GRANT ALL ON SCHEMA public TO chat;
    GRANT ALL ON SCHEMA public TO public;
    COMMENT ON SCHEMA public IS 'standard public schema';
`)
	Logger.Warn("Recreating database")
	if err != nil {
		Logger.Panicf("Error during dropping db: %v", err)
	}
}
