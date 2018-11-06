package facter

import (
	"encoding/json"
	"net"
	"os/exec"

	"github.com/joho/godotenv"
)

type Facts struct {
	OS           OS
	BlockDevices []BlockDevices
	Interfaces   []net.Interface
}

type OS struct {
	Family   string
	ID       string
	Release  string
	Codename string
}

// BlockDevices list of all the blockdevices
type BlockDevices struct {
	Name       string
	KernelName string         `json:"kname"`
	MajMin     string         `json:"maj:min"`
	FsType     string         `json:"fstype,omitempty"`
	Mountpoint string         `json:"mountpoint,omitempty"`
	Label      string         `json:"label,omitempty"`
	UUID       string         `json:"uuid,omitempty"`
	Removable  string         `json:"rm"`
	ReadOnly   string         `json:"ro"`
	Size       string         `json:"size"`
	Type       string         `json:"type"`
	Children   []BlockDevices `json:"children,omitempty"`
}

// New returns a new facter
func New() (*Facts, error) {
	facts := &Facts{}

	osf, err := NewOS()
	if err != nil {
		return nil, err
	}
	facts.OS = osf

	devs, err := NewBlockDevices()
	if err != nil {
		return nil, err
	}
	facts.BlockDevices = devs

	ifaces, err := NewInterfaces()
	if err != nil {
		return nil, err
	}
	facts.Interfaces = ifaces

	return facts, nil
}

// NewOS returns new OS
func NewOS() (OS, error) {
	osf := OS{}

	lsb, err := godotenv.Read("/etc/os-release")
	if err != nil {
		return osf, err
	}

	osf.Family = lsb["ID_LIKE"]
	osf.ID = lsb["ID"]
	osf.Release = lsb["VERSION_ID"]
	osf.Codename = lsb["VERSION_CODENAME"]

	return osf, nil
}

// NewBlockDevices lists all blockdevices
func NewBlockDevices() ([]BlockDevices, error) {
	devs := &struct {
		BlockDevices []BlockDevices `json:"blockdevices"`
	}{}

	out, err := exec.Command("lsblk", "-OJ").Output()
	if err != nil {
		return devs.BlockDevices, err
	}

	if err := json.Unmarshal(out, devs); err != nil {
		return devs.BlockDevices, err
	}

	return devs.BlockDevices, nil
}

func NewInterfaces() ([]net.Interface, error) {
	return net.Interfaces()
}
