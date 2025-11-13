package services

import (
	"bytes"
	"errors"
	"fmt"
	"remote-provider/internal/provider/filesystem"
	"remote-provider/internal/provider/servers"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHService struct {
	connections []SSHConnection
}

func createSSHClient(host *servers.Server) (*ssh.Client, error) {
	conf := &ssh.ClientConfig{
		User:            host.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{},
		Timeout:         10 * time.Second,
	}

	if len(host.Password) > 0 {
		conf.Auth = append(conf.Auth, ssh.Password(host.Password))
	}

	if len(host.PrivateKeyPath) > 0 {
		keyFile, err := filesystem.ReadFile(host.PrivateKeyPath)
		if err != nil && host.Password == "" {
			fmt.Println(err.Error())
			return nil, err
		}

		var signer ssh.Signer

		signer, err = ssh.ParsePrivateKey(keyFile)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}

		conf.Auth = append(conf.Auth, ssh.PublicKeys(signer))
	}

	client, err := ssh.Dial("tcp", host.GetFullAddress(), conf)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return client, nil
}

func NewSSHService(hosts []*servers.Server) *SSHService {
	var connections []SSHConnection
	for _, host := range hosts {
		var foundHost *servers.Server
		for _, connection := range connections {
			if connection.host.Name == host.Name {
				foundHost = connection.host
				break
			}
		}
		if foundHost != nil {
			continue
		}

		client, err := createSSHClient(host)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		connection := SSHConnection{
			host:   host,
			client: client,
		}
		connections = append(connections, connection)
	}

	return &SSHService{
		connections: connections,
	}
}

func (service *SSHService) OpenConnection(host *servers.Server) error {
	var foundHost *servers.Server
	if service.connections == nil {
		service.connections = []SSHConnection{}
	}
	for _, connection := range service.connections {
		if connection.host.Name == host.Name {
			foundHost = connection.host
			break
		}
	}
	if foundHost != nil {
		return nil
	}

	client, err := createSSHClient(host)
	if err != nil {
		return err
	}
	connection := SSHConnection{
		host:   host,
		client: client,
	}
	service.connections = append(service.connections, connection)
	return nil
}

func (service *SSHService) spawnSession(connection *SSHConnection) (*ssh.Session, error) {
	var err error

	var session *ssh.Session
	session, err = connection.client.NewSession()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return session, nil
}

func extractSudoPasswordFromOutput(stdout *bytes.Buffer, password *string) {
	commandOutput := strings.Split(stdout.String(), "\n")
	if strings.Contains(stdout.String(), "[sudo] password for") {
		var filteredOutput []string
		for _, line := range commandOutput {
			if !strings.Contains(line, *password) && !strings.Contains(line, "[sudo] password for") {
				filteredOutput = append(filteredOutput, line)
			}
		}
		commandOutput = filteredOutput
	} else {
		if len(commandOutput) > 0 && strings.Contains(commandOutput[0], *password) {
			commandOutput = commandOutput[1:]
		}
	}
	stdout.Reset()
	stdout.WriteString(strings.Join(commandOutput, "\n"))
}

func (service *SSHService) ExecuteCommand(command string, server *servers.Server) (*servers.ServerCommand, error) {
	var connection *SSHConnection
	for _, conn := range service.connections {
		if conn.host.Name == server.Name {
			connection = &conn
			break
		}
	}

	if connection == nil {
		return nil, fmt.Errorf("no connection found for server %s", server.Name)
	}

	session, err := service.spawnSession(connection)
	if err != nil {
		return nil, err
	}

	defer func(session *ssh.Session) {
		err := session.Close()
		if err != nil && err.Error() != "EOF" {
			fmt.Println(err.Error())
		}
	}(session)

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	session.Stdin = strings.NewReader(connection.host.SudoPassword + "\n")

	err = session.RequestPty("xterm", 40, 80, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400})
	if err != nil {
		return nil, err
	}

	err = session.Run(command)
	extractSudoPasswordFromOutput(&stdout, &connection.host.SudoPassword)

	serverCommand := &servers.ServerCommand{
		Command:  command,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: extractExitCode(err),
	}
	server.History = append(server.History, serverCommand)
	return serverCommand, err
}

func extractExitCode(err error) int8 {
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return int8(exitErr.ExitStatus())
	}
	return 0
}

func (service *SSHService) CloseConnection(connection *SSHConnection) error {
	err := connection.client.Close()
	return err
}

func (service *SSHService) GetConnections() *[]SSHConnection {
	return &service.connections
}
