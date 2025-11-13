package services

import (
	"golang.org/x/crypto/ssh"
	"remote-provider/internal/provider/servers"
)

type Service interface {
	ExecuteCommand(command string, server *servers.Server) (*servers.ServerCommand, error)
}

type SSHConnection struct {
	host   *servers.Server
	client *ssh.Client
}
