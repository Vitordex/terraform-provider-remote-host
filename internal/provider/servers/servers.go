package servers

import (
	"fmt"
	"strconv"
)

type Server struct {
	Name           string
	Address        string
	Port           uint16
	User           string
	Password       string
	PrivateKeyPath string
	SudoPassword   string
	Args           map[string]any
	Err            error
	History        []*ServerCommand
}

func (s *Server) GetFullAddress() string {
	return fmt.Sprintf("%s:%s", s.Address, strconv.Itoa(int(s.Port)))
}

type ServerGroup struct {
	Name    string
	Servers []*Server
	Args    map[string]any
}

type ServerCommand struct {
	server   *Server
	Command  string
	Stdout   string
	Stderr   string
	ExitCode int8
}
