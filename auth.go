package sndotfiles

import (
	"fmt"
	"log"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/jonhadfield/gosn"
	"github.com/spf13/viper"
)

func CliSignIn(email, password, apiServer string) (session gosn.Session, err error) {
	sInput := gosn.SignInInput{
		Email:     email,
		Password:  password,
		APIServer: apiServer,
	}
	sOutput, signInErr := gosn.SignIn(sInput)
	if signInErr != nil {
		if signInErr.Error() == "requestMFA" {
			var tokenValue string
			fmt.Print("token: ")
			_, err = fmt.Scanln(&tokenValue)
			if err != nil {
				return
			}
			sInput.TokenName = sOutput.TokenName
			sInput.TokenVal = strings.TrimSpace(tokenValue)
			sOutput, signInErr = gosn.SignIn(sInput)

			session = sOutput.Session
			if signInErr != nil {
				err = signInErr
				return
			}

		} else {
			log.Fatal(signInErr.Error())
		}
	}
	session = sOutput.Session
	return session, err
}

func GetCredentials(inServer string) (email, password, apiServer, errMsg string) {
	switch {
	case viper.GetString("email") != "":
		email = viper.GetString("email")
	default:
		fmt.Print("email: ")
		_, err := fmt.Scanln(&email)
		if err != nil || len(strings.TrimSpace(email)) == 0 {
			errMsg = "email required"
			return
		}
	}

	if viper.GetString("password") != "" {
		password = viper.GetString("password")
	} else {
		fmt.Print("password: ")
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err == nil {
			password = string(bytePassword)
		} else {
			errMsg = err.Error()
			return
		}
		if strings.TrimSpace(password) == "" {
			errMsg = "password not defined"
		}
	}

	switch {
	case inServer != "":
		apiServer = inServer
	case viper.GetString("server") != "":
		apiServer = viper.GetString("server")
	default:
		apiServer = SNServerURL
	}
	return email, password, apiServer, errMsg
}
