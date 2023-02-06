package main

import (
	"fmt"
	"log"
	"os"
	"ssh/pkg/config"
	"ssh/pkg/server"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func handleShell(sshConn *ssh.ServerConn, channel ssh.Channel) {
	login := sshConn.Permissions.Extensions["login"]
	terminal := term.NewTerminal(channel, login+"> ")
	for {
		line, err := terminal.ReadLine()
		if err != nil {
			log.Println("Error reading line", err)
			break
		}
		fmt.Println(line)
		terminal.Write([]byte("You wrote: " + line + "\n"))
	}
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}

func main() {
	config := config.NewConfig().SshConfig()

	privateBytes, err := os.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)

	server := server.NewServer(config, handleShell)
	server.ListenAndServe(":2222")
}
