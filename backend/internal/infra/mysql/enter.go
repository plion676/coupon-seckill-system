package mysql

import (
	"log/slog"

	sqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	dsn := "root:123456@tcp(127.0.0.1:3306)/practice?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("mysql connect failed", "module", "mysql", "dsn", sanitizeDSN(dsn), "err", err)
		return
	}
	DB = db
	slog.Info("mysql connected", "module", "mysql", "dsn", sanitizeDSN(dsn))
}

func sanitizeDSN(dsn string) string {
	parsed, err := sqldriver.ParseDSN(dsn)
	if err != nil {
		return "invalid_dsn"
	}
	if parsed.Passwd != "" {
		parsed.Passwd = "***"
	}
	return parsed.FormatDSN()
}
