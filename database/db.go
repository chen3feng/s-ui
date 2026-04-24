package database

import (
	"encoding/json"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/util/common"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// hashPassword hashes a plaintext password using bcrypt.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func initUser() error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		// Generate a random password for the initial admin user
		randomPassword := common.Random(16)
		hashedPassword, err := hashPassword(randomPassword)
		if err != nil {
			return err
		}
		user := &model.User{
			Username: "admin",
			Password: hashedPassword,
		}
		err = db.Create(user).Error
		if err != nil {
			return err
		}
		log.Println("============================================")
		log.Println(" Initial admin credentials generated")
		log.Println(" Username: admin")
		log.Println(" Password: (encrypted, use 's-ui admin reset' to set a custom password)")
		log.Println(" Please change the password after login!")
		log.Println("============================================")
	}
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, 01740)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL"
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func InitDB(dbPath string) error {
	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
	)
	if err != nil {
		return err
	}
	err = initUser()
	if err != nil {
		return err
	}

	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
