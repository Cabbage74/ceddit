package mysql

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var db *sqlx.DB

func Init() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
		viper.GetString("mysql.user"),
		viper.GetString("mysql.password"),
		viper.GetString("mysql.host"),
		viper.GetInt("mysql.port"),
		viper.GetString("mysql.dbname"),
	)
	var err error
	db, err = sqlx.Connect("mysql", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(viper.GetInt("mysql.max_open_conns"))
	db.SetMaxIdleConns(viper.GetInt("mysql.max_idle_conns"))

	if viper.GetBool("mysql.auto_migrate") {
		if err := autoMigrate(viper.GetString("mysql.migrate_file")); err != nil {
			return fmt.Errorf("auto migrate: %w", err)
		}
	}

	return nil
}

func autoMigrate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migrate file %s: %w", path, err)
	}

	statements := strings.Split(string(data), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec: %s: %w", truncate(stmt, 80), err)
		}
	}

	zap.L().Info("auto migrate completed")
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// DB returns the underlying *sql.DB for use by subsystems (e.g. Kafka consumer).
func DB() *sqlx.DB {
	return db
}

func Close() {
	_ = db.Close()
}
