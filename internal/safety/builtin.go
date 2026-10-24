package safety

// builtin classifies every method of the org.varlink.service and io.systemd.*
// interfaces in systemd v257–v262, so that systemd calls never depend on the
// name heuristic. TestBuiltinCoversSystemd keeps it complete.
var builtin = map[string]Class{
	// Introspection.
	"org.varlink.service.GetInfo":                 Read,
	"org.varlink.service.GetInterfaceDescription": Read,

	"io.systemd.AskPassword.Ask": Write, // prompts the user on a console

	"io.systemd.BootControl.ListBootEntries":     Read,
	"io.systemd.BootControl.GetRebootToFirmware": Read,
	"io.systemd.BootControl.SetRebootToFirmware": Danger,
	"io.systemd.BootControl.Install":             Danger,
	"io.systemd.BootControl.Link":                Danger,
	"io.systemd.BootControl.LinkAuto":            Danger,
	"io.systemd.BootControl.Unlink":              Danger,

	"io.systemd.Credentials.Encrypt": Write,
	"io.systemd.Credentials.Decrypt": Write, // reveals secrets

	"io.systemd.CryptEnroll.ListSlots": Read,
	"io.systemd.CryptEnroll.Enroll":    Danger, // adds a way to unlock the disk

	"io.systemd.FactoryReset.GetFactoryResetMode":    Read,
	"io.systemd.FactoryReset.CanRequestFactoryReset": Read,

	"io.systemd.Hostname.Describe":          Read,
	"io.systemd.Hostname.SetHostname":       Write,
	"io.systemd.Hostname.SetStaticHostname": Write,
	"io.systemd.Hostname.SetPrettyHostname": Write,
	"io.systemd.Hostname.SetIconName":       Write,
	"io.systemd.Hostname.SetChassis":        Write,
	"io.systemd.Hostname.SetDeployment":     Write,
	"io.systemd.Hostname.SetLocation":       Write,
	"io.systemd.Hostname.SetTags":           Write,

	"io.systemd.Import.ListTransfers": Read,
	"io.systemd.Import.Pull":          Write,

	"io.systemd.InstanceMetadata.GetVendorInfo": Read,
	"io.systemd.InstanceMetadata.Get":           Write, // user data can hold secrets

	"io.systemd.Job.List":     Read,
	"io.systemd.Job.Cancel":   Write,
	"io.systemd.Job.ClearAll": Danger,

	"io.systemd.Journal.Synchronize":   Write,
	"io.systemd.Journal.Rotate":        Write,
	"io.systemd.Journal.FlushToVar":    Write,
	"io.systemd.Journal.RelinquishVar": Write,

	"io.systemd.JournalAccess.GetEntries": Read,

	"io.systemd.Login.CreateSession":  Write,
	"io.systemd.Login.ReleaseSession": Write,
	"io.systemd.Login.ListSessions":   Read,
	"io.systemd.Login.ListUsers":      Read,
	"io.systemd.Login.ListSeats":      Read,
	"io.systemd.Login.ListInhibitors": Read,

	"io.systemd.Machine.Register":          Write,
	"io.systemd.Machine.Unregister":        Danger,
	"io.systemd.Machine.Terminate":         Danger,
	"io.systemd.Machine.Kill":              Danger,
	"io.systemd.Machine.List":              Read,
	"io.systemd.Machine.Open":              Danger, // opens a shell or login in the machine
	"io.systemd.Machine.MapFrom":           Read,
	"io.systemd.Machine.MapTo":             Read,
	"io.systemd.Machine.BindMount":         Write,
	"io.systemd.Machine.CopyFrom":          Write,
	"io.systemd.Machine.CopyTo":            Danger,
	"io.systemd.Machine.OpenRootDirectory": Write,

	"io.systemd.MachineImage.List":         Read,
	"io.systemd.MachineImage.Update":       Write,
	"io.systemd.MachineImage.Clone":        Write,
	"io.systemd.MachineImage.Remove":       Danger,
	"io.systemd.MachineImage.SetPoolLimit": Write,
	"io.systemd.MachineImage.CleanPool":    Danger,

	"io.systemd.MachineInstance.Describe":        Read,
	"io.systemd.MachineInstance.SubscribeEvents": Read,
	"io.systemd.MachineInstance.Pause":           Danger, // freezes the machine until Resume
	"io.systemd.MachineInstance.Resume":          Write,
	"io.systemd.MachineInstance.PowerOff":        Danger,
	"io.systemd.MachineInstance.Reboot":          Danger,
	"io.systemd.MachineInstance.Terminate":       Danger,
	"io.systemd.MachineInstance.AddStorage":      Write,
	"io.systemd.MachineInstance.RemoveStorage":   Danger, // pulls a disk from a running machine
	"io.systemd.MachineInstance.ReplaceStorage":  Danger,

	"io.systemd.ManagedOOM.SubscribeManagedOOMCGroups": Read,

	"io.systemd.Manager.Describe":          Read,
	"io.systemd.Manager.Reexecute":         Danger,
	"io.systemd.Manager.Reload":            Danger,
	"io.systemd.Manager.EnqueueMarkedJobs": Danger,
	"io.systemd.Manager.PowerOff":          Danger,
	"io.systemd.Manager.Reboot":            Danger,
	"io.systemd.Manager.Halt":              Danger,
	"io.systemd.Manager.KExec":             Danger,
	"io.systemd.Manager.SoftReboot":        Danger,

	"io.systemd.Metrics.List":     Read,
	"io.systemd.Metrics.Describe": Read,

	"io.systemd.MountFileSystem.MountImage":     Write,
	"io.systemd.MountFileSystem.MountDirectory": Write,
	"io.systemd.MountFileSystem.MakeDirectory":  Write,

	"io.systemd.MuteConsole.Mute": Write,

	"io.systemd.NamespaceResource.AllocateUserRange":              Write,
	"io.systemd.NamespaceResource.RegisterUserNamespace":          Write,
	"io.systemd.NamespaceResource.AddMountToUserNamespace":        Write,
	"io.systemd.NamespaceResource.AddControlGroupToUserNamespace": Write,
	"io.systemd.NamespaceResource.AddNetworkToUserNamespace":      Write,

	"io.systemd.Network.Describe":             Read,
	"io.systemd.Network.GetStates":            Read,
	"io.systemd.Network.GetNamespaceId":       Read,
	"io.systemd.Network.GetLLDPNeighbors":     Read,
	"io.systemd.Network.SetPersistentStorage": Write,
	"io.systemd.Network.Reload":               Danger, // reconfigures every link by default

	"io.systemd.Network.Link.Describe":    Read,
	"io.systemd.Network.Link.Up":          Danger, // can cut the network you are using
	"io.systemd.Network.Link.Down":        Danger,
	"io.systemd.Network.Link.Reconfigure": Danger,
	"io.systemd.Network.Link.Renew":       Write,
	"io.systemd.Network.Link.ForceRenew":  Write,

	"io.systemd.oom.ReportManagedOOMCGroups": Write,
	"io.systemd.oom.Prekill.Notify":          Write,

	"io.systemd.PCRExtend.Extend": Danger, // irreversible until reboot

	"io.systemd.PCRLock.ReadEventLog":   Read,
	"io.systemd.PCRLock.MakePolicy":     Danger,
	"io.systemd.PCRLock.RemovePolicy":   Danger,
	"io.systemd.PCRLock.ListComponents": Read,
	"io.systemd.PCRLock.Lock":           Danger,

	"io.systemd.Repart.Run":                  Danger,
	"io.systemd.Repart.ListCandidateDevices": Read,

	"io.systemd.Report.Generate":        Read,
	"io.systemd.Report.GenerateSigned":  Write,  // uses the machine's signing keys
	"io.systemd.Report.Signer.Sign":     Danger, // signs any digest with the machine's keys
	"io.systemd.Report.Uploader.Upload": Write,  // sends data off the machine

	"io.systemd.Resolve.ResolveHostname":      Read,
	"io.systemd.Resolve.ResolveAddress":       Read,
	"io.systemd.Resolve.ResolveService":       Read,
	"io.systemd.Resolve.ResolveRecord":        Read,
	"io.systemd.Resolve.BrowseServices":       Read,
	"io.systemd.Resolve.DumpDNSConfiguration": Read,

	"io.systemd.Resolve.Hook.QueryFilter":   Read,
	"io.systemd.Resolve.Hook.ResolveRecord": Read,

	"io.systemd.Resolve.Monitor.SubscribeQueryResults":     Read,
	"io.systemd.Resolve.Monitor.DumpCache":                 Read,
	"io.systemd.Resolve.Monitor.DumpServerState":           Read,
	"io.systemd.Resolve.Monitor.DumpStatistics":            Read,
	"io.systemd.Resolve.Monitor.ResetStatistics":           Write,
	"io.systemd.Resolve.Monitor.FlushCaches":               Write,
	"io.systemd.Resolve.Monitor.ResetServerFeatures":       Write,
	"io.systemd.Resolve.Monitor.SubscribeDNSConfiguration": Read,

	"io.systemd.service.Ping":           Read,
	"io.systemd.service.Reload":         Write,
	"io.systemd.service.SetLogLevel":    Write,
	"io.systemd.service.GetLogLevel":    Read,
	"io.systemd.service.GetEnvironment": Read,

	"io.systemd.Shutdown.PowerOff":   Danger,
	"io.systemd.Shutdown.Reboot":     Danger,
	"io.systemd.Shutdown.Halt":       Danger,
	"io.systemd.Shutdown.KExec":      Danger,
	"io.systemd.Shutdown.SoftReboot": Danger,

	"io.systemd.StorageProvider.ListVolumes":   Read,
	"io.systemd.StorageProvider.ListTemplates": Read,
	"io.systemd.StorageProvider.Acquire":       Write, // can create a volume

	"io.systemd.sysext.Merge":   Danger,
	"io.systemd.sysext.Unmerge": Danger,
	"io.systemd.sysext.Refresh": Danger,
	"io.systemd.sysext.List":    Read,

	"io.systemd.SysInstall.ListCandidateDevices": Read,
	"io.systemd.SysInstall.Run":                  Danger, // installs an OS, can erase the disk

	"io.systemd.SysUpdate.ListTargets":              Read,
	"io.systemd.SysUpdate.ListFeatures":             Read,
	"io.systemd.SysUpdate.CheckNew":                 Read,
	"io.systemd.SysUpdate.Notify.OnCompletedUpdate": Write,

	"io.systemd.Udev.SetTrace":       Write,
	"io.systemd.Udev.SetChildrenMax": Write,
	"io.systemd.Udev.SetEnvironment": Write,
	"io.systemd.Udev.Revert":         Write,
	"io.systemd.Udev.StartExecQueue": Write,
	"io.systemd.Udev.StopExecQueue":  Danger, // stalls device handling
	"io.systemd.Udev.Exit":           Danger,

	"io.systemd.Unit.List":           Read,
	"io.systemd.Unit.SetProperties":  Write,
	"io.systemd.Unit.StartTransient": Danger, // runs any command as the manager's user

	"io.systemd.UserDatabase.GetUserRecord":  Read,
	"io.systemd.UserDatabase.GetGroupRecord": Read,
	"io.systemd.UserDatabase.GetMemberships": Read,
}
