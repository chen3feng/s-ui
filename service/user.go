package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
}

// hashPassword hashes a plaintext password using bcrypt.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// verifyPassword checks if a plaintext password matches a bcrypt hash.
func verifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// isBcryptHash checks if a string looks like a bcrypt hash.
func isBcryptHash(s string) bool {
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$")
}

// MigratePlaintextPasswords checks all users and migrates plaintext passwords to bcrypt hashes.
func (s *UserService) MigratePlaintextPasswords() error {
	db := database.GetDB()
	var users []model.User
	err := db.Model(model.User{}).Find(&users).Error
	if err != nil {
		return err
	}
	for _, user := range users {
		if !isBcryptHash(user.Password) {
			hashed, err := hashPassword(user.Password)
			if err != nil {
				return err
			}
			err = db.Model(model.User{}).Where("id = ?", user.Id).Update("password", hashed).Error
			if err != nil {
				return err
			}
			logger.Info("Migrated password to bcrypt for user: ", user.Username)
		}
	}
	return nil
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return common.NewError("username can not be empty")
	} else if password == "" {
		return common.NewError("password can not be empty")
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	return db.Save(user).Error
}

func (s *UserService) Login(username string, password string, remoteIP string) (string, error) {
	user := s.CheckUser(username, password, remoteIP)
	if user == nil {
		return "", common.NewError("wrong user or password! IP: ", remoteIP)
	}
	return user.Username, nil
}

func (s *UserService) CheckUser(username string, password string, remoteIP string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if database.IsNotFound(err) {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err, " IP: ", remoteIP)
		return nil
	}

	// Verify password using bcrypt
	if !verifyPassword(user.Password, password) {
		return nil
	}

	lastLoginTxt := time.Now().Format("2006-01-02 15:04:05") + " " + remoteIP
	err = db.Model(model.User{}).
		Where("username = ?", username).
		Update("last_logins", &lastLoginTxt).Error
	if err != nil {
		logger.Warning("unable to log login data", err)
	}
	return user
}

func (s *UserService) GetUsers() (*[]model.User, error) {
	var users []model.User
	db := database.GetDB()
	err := db.Model(model.User{}).Select("id,username,last_logins").Scan(&users).Error
	if err != nil {
		return nil, err
	}
	return &users, nil
}

func (s *UserService) ChangePass(id string, oldPass string, newUser string, newPass string) error {
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).Where("id = ?", id).First(user).Error
	if err != nil || database.IsNotFound(err) {
		return err
	}

	// Verify old password using bcrypt
	if !verifyPassword(user.Password, oldPass) {
		return common.NewError("old password is incorrect")
	}

	// Hash new password
	hashedPassword, err := hashPassword(newPass)
	if err != nil {
		return err
	}

	user.Username = newUser
	user.Password = hashedPassword
	return db.Save(user).Error
}

func (s *UserService) LoadTokens() ([]byte, error) {
	db := database.GetDB()
	var tokens []model.Tokens
	err := db.Model(model.Tokens{}).Preload("User").Where("expiry == 0 or expiry > ?", time.Now().Unix()).Find(&tokens).Error
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, t := range tokens {
		result = append(result, map[string]interface{}{
			"token":    t.Token,
			"expiry":   t.Expiry,
			"username": t.User.Username,
		})
	}
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	return jsonResult, nil
}

func (s *UserService) GetUserTokens(username string) (*[]model.Tokens, error) {
	db := database.GetDB()
	var token []model.Tokens
	err := db.Model(model.Tokens{}).Select("id,desc,'****' as token,expiry,user_id").Where("user_id = (select id from users where username = ?)", username).Find(&token).Error
	if err != nil && !database.IsNotFound(err) {
		println(err.Error())
		return nil, err
	}
	return &token, nil
}

func (s *UserService) AddToken(username string, expiry int64, desc string) (string, error) {
	db := database.GetDB()
	var userId uint
	err := db.Model(model.User{}).Where("username = ?", username).Select("id").Scan(&userId).Error
	if err != nil {
		return "", err
	}
	if expiry > 0 {
		expiry = expiry*86400 + time.Now().Unix()
	}
	token := &model.Tokens{
		Token:  common.Random(32),
		Desc:   desc,
		Expiry: expiry,
		UserId: userId,
	}
	err = db.Create(token).Error
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

func (s *UserService) DeleteToken(id string) error {
	db := database.GetDB()
	return db.Model(model.Tokens{}).Where("id = ?", id).Delete(&model.Tokens{}).Error
}
