package cmd

import (
	"fmt"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"
)

func resetAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	// Generate a random password for the reset
	randomPassword := common.Random(16)
	userService := service.UserService{}
	err = userService.UpdateFirstUser("admin", randomPassword)
	if err != nil {
		fmt.Println("reset admin credentials failed:", err)
	} else {
		fmt.Println("============================================")
		fmt.Println(" Admin credentials have been reset")
		fmt.Println(" Username: admin")
		fmt.Println(" Password:", randomPassword)
		fmt.Println(" Please change the password after login!")
		fmt.Println("============================================")
	}
}

func updateAdmin(username string, password string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	if username != "" || password != "" {
		userService := service.UserService{}
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("reset admin credentials failed:", err)
		} else {
			fmt.Println("reset admin credentials success")
		}
	}
}

func showAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	userService := service.UserService{}
	userModel, err := userService.GetFirstUser()
	if err != nil {
		fmt.Println("get current user info failed,error info:", err)
	}
	username := userModel.Username
	if username == "" {
		fmt.Println("current username is empty")
	}
	fmt.Println("First admin credentials:")
	fmt.Println("\tUsername:\t", username)
	fmt.Println("\tPassword:\t (encrypted, cannot be displayed)")
}
