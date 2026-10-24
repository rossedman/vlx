package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rossedman/vlx/internal/discovery"
)

// Service groups on the home page, in display order.
const (
	groupSystem   = "System"
	groupNetwork  = "Network"
	groupUsers    = "Users & login"
	groupMachines = "Machines & images"
	groupStorage  = "Storage"
	groupSecurity = "Security"
	groupDevices  = "Devices"
	groupSession  = "Your session"
	groupDemo     = "Demo"
	groupOther    = "Other"
)

var groupOrder = []string{
	groupSystem, groupNetwork, groupUsers, groupMachines, groupStorage,
	groupSecurity, groupDevices, groupSession, groupDemo, groupOther,
}

type catalogEntry struct {
	group string
	desc  string
}

// catalog describes the sockets systemd ships, by socket name, so the home
// page can say what each service is for before anything is sent to it.
var catalog = map[string]catalogEntry{
	"io.systemd.Hostname":          {groupSystem, "Host name, operating system and hardware"},
	"io.systemd.Manager":           {groupSystem, "The service manager (PID 1): units and jobs"},
	"io.systemd.journal":           {groupSystem, "The journal: flush, rotate and sync logs"},
	"io.systemd.JournalAccess":     {groupSystem, "Read journal entries"},
	"io.systemd.AskPassword":       {groupSystem, "Password prompts from the system"},
	"io.systemd.BootControl":       {groupSystem, "Boot loader entries and reboot-to-firmware"},
	"io.systemd.FactoryReset":      {groupSystem, "Factory reset state"},
	"io.systemd.ManagedOOM":        {groupSystem, "Out-of-memory killing policy"},
	"io.systemd.MuteConsole":       {groupSystem, "Quiet the kernel console"},
	"io.systemd.Shutdown":          {groupSystem, "Power off, reboot and halt"},
	"io.systemd.SysUpdate":         {groupSystem, "OS updates: targets and new versions"},
	"io.systemd.SysInstall":        {groupSystem, "Install the OS onto a disk"},
	"io.systemd.Report":            {groupSystem, "Reports of the system's metrics"},
	"io.systemd.InstanceMetadata":  {groupSystem, "Cloud instance metadata (IMDS)"},
	"io.systemd.Resolve":           {groupNetwork, "DNS lookups and resolver settings"},
	"io.systemd.Resolve.Monitor":   {groupNetwork, "Watch DNS queries as they happen"},
	"io.systemd.Network":           {groupNetwork, "Links, addresses and LLDP neighbours"},
	"io.systemd.Multiplexer":       {groupUsers, "Every user and group, from all user databases"},
	"io.systemd.Login":             {groupUsers, "Sessions and seats"},
	"io.systemd.Machine":           {groupMachines, "Containers and virtual machines"},
	"io.systemd.MachineImage":      {groupMachines, "Disk images for machines"},
	"io.systemd.Import":            {groupMachines, "Download and import machine images"},
	"io.systemd.NamespaceResource": {groupMachines, "User namespaces for unprivileged containers"},
	"io.systemd.MountFileSystem":   {groupStorage, "Mount disk images and directories"},
	"io.systemd.Repart":            {groupStorage, "Grow and add partitions"},
	"io.systemd.sysext":            {groupStorage, "System and configuration extensions"},
	"io.systemd.Credentials":       {groupSecurity, "Encrypt and decrypt credentials"},
	"io.systemd.CryptEnroll":       {groupSecurity, "Enroll keys into encrypted (LUKS2) disks"},
	"io.systemd.PCRExtend":         {groupSecurity, "Measure into TPM PCRs"},
	"io.systemd.PCRLock":           {groupSecurity, "TPM PCR policies and the event log"},
	"io.systemd.Udev":              {groupDevices, "The device manager"},
	"io.vlx.demo":                  {groupDemo, "vlx's sample service: a greeter and a tiny inventory"},
}

// describe returns the group and description for a socket.
func describe(s discovery.Socket) (group, desc string) {
	dir := filepath.Base(filepath.Dir(s.Path))
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" && strings.HasPrefix(s.Path, filepath.Join(xdg, "systemd")+"/") {
		e, ok := catalog[s.Name]
		if !ok {
			return groupSession, "A service of your user manager"
		}
		return groupSession, e.desc + " (yours)"
	}
	switch dir {
	case "userdb":
		if s.Name == "io.systemd.Multiplexer" {
			break
		}
		return groupUsers, "User and group records from " + discovery.ShortName(s.Name)
	case "report":
		return groupSystem, "Metrics from " + discovery.ShortName(s.Name)
	}
	if e, ok := catalog[s.Name]; ok {
		return e.group, e.desc
	}
	return groupOther, ""
}

// label is the short name shown for a socket.
func label(s discovery.Socket) string {
	name := discovery.ShortName(s.Name)
	switch filepath.Base(filepath.Dir(s.Path)) {
	case "userdb":
		if s.Name != "io.systemd.Multiplexer" {
			return name + " (userdb)"
		}
	case "report":
		return name + " (metrics)"
	}
	return name
}
