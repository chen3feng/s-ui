package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alireza0/s-ui/config"
	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/service"
)

func resetAdmin() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Print("Enter new password: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	newPassword := strings.TrimSpace(scanner.Text())
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading password:", err)
		return
	}
	if newPassword == "" {
		fmt.Println("Password cannot be empty")
		return
	}

	userService := service.UserService{}
	err = userService.UpdateFirstUser("admin", newPassword)
	if err != nil {
		fmt.Println("reset admin credentials failed:", err)
	} else {
		fmt.Println("============================================")
		fmt.Println(" Admin credentials have been reset!")
		fmt.Println(" Username: admin")
		fmt.Println(" Password: (updated, not echoed)")
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
