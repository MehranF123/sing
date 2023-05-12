package control

import (
	"net"

	E "github.com/MehranF123/sing/common/exceptions"
)

type BindManager interface {
	IndexByName(name string) (int, error)
	Update() error
}

type myBindManager struct {
	interfaceIndexByName map[string]int
}

func (m *myBindManager) IndexByName(name string) (int, error) {
	if index, loaded := m.interfaceIndexByName[name]; loaded {
		return index, nil
	}
	err := m.Update()
	if err != nil {
		return 0, err
	}
	if index, loaded := m.interfaceIndexByName[name]; loaded {
		return index, nil
	}
	return 0, E.New("interface ", name, " not found")
}

func (m *myBindManager) Update() error {
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	interfaceIndexByName := make(map[string]int)
	for _, iface := range interfaces {
		interfaceIndexByName[iface.Name] = iface.Index
	}
	m.interfaceIndexByName = interfaceIndexByName
	return nil
}
