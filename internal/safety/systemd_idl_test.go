package safety

// Code generated for tests by listing sdvarlink's systemd packages. DO NOT EDIT by hand;
// regenerate with hack/gen-safety-test.sh when sdvarlink adds a systemd release.

import (
	v257bootcontrol "github.com/rossedman/sdvarlink/systemd/v257/bootcontrol"
	v257credentials "github.com/rossedman/sdvarlink/systemd/v257/credentials"
	v257hostname "github.com/rossedman/sdvarlink/systemd/v257/hostname"
	v257importd "github.com/rossedman/sdvarlink/systemd/v257/importd"
	v257journal "github.com/rossedman/sdvarlink/systemd/v257/journal"
	v257machine "github.com/rossedman/sdvarlink/systemd/v257/machine"
	v257machineimage "github.com/rossedman/sdvarlink/systemd/v257/machineimage"
	v257managedoom "github.com/rossedman/sdvarlink/systemd/v257/managedoom"
	v257mountfilesystem "github.com/rossedman/sdvarlink/systemd/v257/mountfilesystem"
	v257namespaceresource "github.com/rossedman/sdvarlink/systemd/v257/namespaceresource"
	v257network "github.com/rossedman/sdvarlink/systemd/v257/network"
	v257oom "github.com/rossedman/sdvarlink/systemd/v257/oom"
	v257pcrextend "github.com/rossedman/sdvarlink/systemd/v257/pcrextend"
	v257pcrlock "github.com/rossedman/sdvarlink/systemd/v257/pcrlock"
	v257resolve "github.com/rossedman/sdvarlink/systemd/v257/resolve"
	v257resolvemonitor "github.com/rossedman/sdvarlink/systemd/v257/resolvemonitor"
	v257service "github.com/rossedman/sdvarlink/systemd/v257/service"
	v257sysext "github.com/rossedman/sdvarlink/systemd/v257/sysext"
	v257systemd "github.com/rossedman/sdvarlink/systemd/v257/systemd"
	v257userdatabase "github.com/rossedman/sdvarlink/systemd/v257/userdatabase"
	v258askpassword "github.com/rossedman/sdvarlink/systemd/v258/askpassword"
	v258bootcontrol "github.com/rossedman/sdvarlink/systemd/v258/bootcontrol"
	v258credentials "github.com/rossedman/sdvarlink/systemd/v258/credentials"
	v258factoryreset "github.com/rossedman/sdvarlink/systemd/v258/factoryreset"
	v258hostname "github.com/rossedman/sdvarlink/systemd/v258/hostname"
	v258importd "github.com/rossedman/sdvarlink/systemd/v258/importd"
	v258journal "github.com/rossedman/sdvarlink/systemd/v258/journal"
	v258login "github.com/rossedman/sdvarlink/systemd/v258/login"
	v258machine "github.com/rossedman/sdvarlink/systemd/v258/machine"
	v258machineimage "github.com/rossedman/sdvarlink/systemd/v258/machineimage"
	v258managedoom "github.com/rossedman/sdvarlink/systemd/v258/managedoom"
	v258manager "github.com/rossedman/sdvarlink/systemd/v258/manager"
	v258mountfilesystem "github.com/rossedman/sdvarlink/systemd/v258/mountfilesystem"
	v258namespaceresource "github.com/rossedman/sdvarlink/systemd/v258/namespaceresource"
	v258network "github.com/rossedman/sdvarlink/systemd/v258/network"
	v258oom "github.com/rossedman/sdvarlink/systemd/v258/oom"
	v258pcrextend "github.com/rossedman/sdvarlink/systemd/v258/pcrextend"
	v258pcrlock "github.com/rossedman/sdvarlink/systemd/v258/pcrlock"
	v258resolve "github.com/rossedman/sdvarlink/systemd/v258/resolve"
	v258resolvemonitor "github.com/rossedman/sdvarlink/systemd/v258/resolvemonitor"
	v258service "github.com/rossedman/sdvarlink/systemd/v258/service"
	v258sysext "github.com/rossedman/sdvarlink/systemd/v258/sysext"
	v258systemd "github.com/rossedman/sdvarlink/systemd/v258/systemd"
	v258udev "github.com/rossedman/sdvarlink/systemd/v258/udev"
	v258unit "github.com/rossedman/sdvarlink/systemd/v258/unit"
	v258userdatabase "github.com/rossedman/sdvarlink/systemd/v258/userdatabase"
	v260askpassword "github.com/rossedman/sdvarlink/systemd/v260/askpassword"
	v260bootcontrol "github.com/rossedman/sdvarlink/systemd/v260/bootcontrol"
	v260credentials "github.com/rossedman/sdvarlink/systemd/v260/credentials"
	v260factoryreset "github.com/rossedman/sdvarlink/systemd/v260/factoryreset"
	v260hostname "github.com/rossedman/sdvarlink/systemd/v260/hostname"
	v260importd "github.com/rossedman/sdvarlink/systemd/v260/importd"
	v260journal "github.com/rossedman/sdvarlink/systemd/v260/journal"
	v260journalaccess "github.com/rossedman/sdvarlink/systemd/v260/journalaccess"
	v260login "github.com/rossedman/sdvarlink/systemd/v260/login"
	v260machine "github.com/rossedman/sdvarlink/systemd/v260/machine"
	v260machineimage "github.com/rossedman/sdvarlink/systemd/v260/machineimage"
	v260managedoom "github.com/rossedman/sdvarlink/systemd/v260/managedoom"
	v260manager "github.com/rossedman/sdvarlink/systemd/v260/manager"
	v260metrics "github.com/rossedman/sdvarlink/systemd/v260/metrics"
	v260mountfilesystem "github.com/rossedman/sdvarlink/systemd/v260/mountfilesystem"
	v260muteconsole "github.com/rossedman/sdvarlink/systemd/v260/muteconsole"
	v260namespaceresource "github.com/rossedman/sdvarlink/systemd/v260/namespaceresource"
	v260network "github.com/rossedman/sdvarlink/systemd/v260/network"
	v260networklink "github.com/rossedman/sdvarlink/systemd/v260/networklink"
	v260oom "github.com/rossedman/sdvarlink/systemd/v260/oom"
	v260oomprekill "github.com/rossedman/sdvarlink/systemd/v260/oomprekill"
	v260pcrextend "github.com/rossedman/sdvarlink/systemd/v260/pcrextend"
	v260pcrlock "github.com/rossedman/sdvarlink/systemd/v260/pcrlock"
	v260repart "github.com/rossedman/sdvarlink/systemd/v260/repart"
	v260resolve "github.com/rossedman/sdvarlink/systemd/v260/resolve"
	v260resolvehook "github.com/rossedman/sdvarlink/systemd/v260/resolvehook"
	v260resolvemonitor "github.com/rossedman/sdvarlink/systemd/v260/resolvemonitor"
	v260service "github.com/rossedman/sdvarlink/systemd/v260/service"
	v260sysext "github.com/rossedman/sdvarlink/systemd/v260/sysext"
	v260systemd "github.com/rossedman/sdvarlink/systemd/v260/systemd"
	v260udev "github.com/rossedman/sdvarlink/systemd/v260/udev"
	v260unit "github.com/rossedman/sdvarlink/systemd/v260/unit"
	v260userdatabase "github.com/rossedman/sdvarlink/systemd/v260/userdatabase"
	v262askpassword "github.com/rossedman/sdvarlink/systemd/v262/askpassword"
	v262bootcontrol "github.com/rossedman/sdvarlink/systemd/v262/bootcontrol"
	v262credentials "github.com/rossedman/sdvarlink/systemd/v262/credentials"
	v262cryptenroll "github.com/rossedman/sdvarlink/systemd/v262/cryptenroll"
	v262factoryreset "github.com/rossedman/sdvarlink/systemd/v262/factoryreset"
	v262hostname "github.com/rossedman/sdvarlink/systemd/v262/hostname"
	v262importd "github.com/rossedman/sdvarlink/systemd/v262/importd"
	v262instancemetadata "github.com/rossedman/sdvarlink/systemd/v262/instancemetadata"
	v262job "github.com/rossedman/sdvarlink/systemd/v262/job"
	v262journal "github.com/rossedman/sdvarlink/systemd/v262/journal"
	v262journalaccess "github.com/rossedman/sdvarlink/systemd/v262/journalaccess"
	v262login "github.com/rossedman/sdvarlink/systemd/v262/login"
	v262machine "github.com/rossedman/sdvarlink/systemd/v262/machine"
	v262machineimage "github.com/rossedman/sdvarlink/systemd/v262/machineimage"
	v262machineinstance "github.com/rossedman/sdvarlink/systemd/v262/machineinstance"
	v262managedoom "github.com/rossedman/sdvarlink/systemd/v262/managedoom"
	v262manager "github.com/rossedman/sdvarlink/systemd/v262/manager"
	v262metrics "github.com/rossedman/sdvarlink/systemd/v262/metrics"
	v262mountfilesystem "github.com/rossedman/sdvarlink/systemd/v262/mountfilesystem"
	v262muteconsole "github.com/rossedman/sdvarlink/systemd/v262/muteconsole"
	v262namespaceresource "github.com/rossedman/sdvarlink/systemd/v262/namespaceresource"
	v262network "github.com/rossedman/sdvarlink/systemd/v262/network"
	v262networklink "github.com/rossedman/sdvarlink/systemd/v262/networklink"
	v262oom "github.com/rossedman/sdvarlink/systemd/v262/oom"
	v262oomprekill "github.com/rossedman/sdvarlink/systemd/v262/oomprekill"
	v262pcrextend "github.com/rossedman/sdvarlink/systemd/v262/pcrextend"
	v262pcrlock "github.com/rossedman/sdvarlink/systemd/v262/pcrlock"
	v262repart "github.com/rossedman/sdvarlink/systemd/v262/repart"
	v262report "github.com/rossedman/sdvarlink/systemd/v262/report"
	v262reportsigner "github.com/rossedman/sdvarlink/systemd/v262/reportsigner"
	v262reportuploader "github.com/rossedman/sdvarlink/systemd/v262/reportuploader"
	v262resolve "github.com/rossedman/sdvarlink/systemd/v262/resolve"
	v262resolvehook "github.com/rossedman/sdvarlink/systemd/v262/resolvehook"
	v262resolvemonitor "github.com/rossedman/sdvarlink/systemd/v262/resolvemonitor"
	v262service "github.com/rossedman/sdvarlink/systemd/v262/service"
	v262shutdown "github.com/rossedman/sdvarlink/systemd/v262/shutdown"
	v262storageprovider "github.com/rossedman/sdvarlink/systemd/v262/storageprovider"
	v262sysext "github.com/rossedman/sdvarlink/systemd/v262/sysext"
	v262sysinstall "github.com/rossedman/sdvarlink/systemd/v262/sysinstall"
	v262systemd "github.com/rossedman/sdvarlink/systemd/v262/systemd"
	v262sysupdate "github.com/rossedman/sdvarlink/systemd/v262/sysupdate"
	v262sysupdatenotify "github.com/rossedman/sdvarlink/systemd/v262/sysupdatenotify"
	v262udev "github.com/rossedman/sdvarlink/systemd/v262/udev"
	v262unit "github.com/rossedman/sdvarlink/systemd/v262/unit"
	v262userdatabase "github.com/rossedman/sdvarlink/systemd/v262/userdatabase"
	v262virtualmachineinstance "github.com/rossedman/sdvarlink/systemd/v262/virtualmachineinstance"
)

var systemdIDL = []string{
	v257bootcontrol.InterfaceDescription,
	v257credentials.InterfaceDescription,
	v257hostname.InterfaceDescription,
	v257importd.InterfaceDescription,
	v257journal.InterfaceDescription,
	v257machine.InterfaceDescription,
	v257machineimage.InterfaceDescription,
	v257managedoom.InterfaceDescription,
	v257mountfilesystem.InterfaceDescription,
	v257namespaceresource.InterfaceDescription,
	v257network.InterfaceDescription,
	v257oom.InterfaceDescription,
	v257pcrextend.InterfaceDescription,
	v257pcrlock.InterfaceDescription,
	v257resolve.InterfaceDescription,
	v257resolvemonitor.InterfaceDescription,
	v257service.InterfaceDescription,
	v257sysext.InterfaceDescription,
	v257systemd.InterfaceDescription,
	v257userdatabase.InterfaceDescription,
	v258askpassword.InterfaceDescription,
	v258bootcontrol.InterfaceDescription,
	v258credentials.InterfaceDescription,
	v258factoryreset.InterfaceDescription,
	v258hostname.InterfaceDescription,
	v258importd.InterfaceDescription,
	v258journal.InterfaceDescription,
	v258login.InterfaceDescription,
	v258machine.InterfaceDescription,
	v258machineimage.InterfaceDescription,
	v258managedoom.InterfaceDescription,
	v258manager.InterfaceDescription,
	v258mountfilesystem.InterfaceDescription,
	v258namespaceresource.InterfaceDescription,
	v258network.InterfaceDescription,
	v258oom.InterfaceDescription,
	v258pcrextend.InterfaceDescription,
	v258pcrlock.InterfaceDescription,
	v258resolve.InterfaceDescription,
	v258resolvemonitor.InterfaceDescription,
	v258service.InterfaceDescription,
	v258sysext.InterfaceDescription,
	v258systemd.InterfaceDescription,
	v258udev.InterfaceDescription,
	v258unit.InterfaceDescription,
	v258userdatabase.InterfaceDescription,
	v260askpassword.InterfaceDescription,
	v260bootcontrol.InterfaceDescription,
	v260credentials.InterfaceDescription,
	v260factoryreset.InterfaceDescription,
	v260hostname.InterfaceDescription,
	v260importd.InterfaceDescription,
	v260journal.InterfaceDescription,
	v260journalaccess.InterfaceDescription,
	v260login.InterfaceDescription,
	v260machine.InterfaceDescription,
	v260machineimage.InterfaceDescription,
	v260managedoom.InterfaceDescription,
	v260manager.InterfaceDescription,
	v260metrics.InterfaceDescription,
	v260mountfilesystem.InterfaceDescription,
	v260muteconsole.InterfaceDescription,
	v260namespaceresource.InterfaceDescription,
	v260network.InterfaceDescription,
	v260networklink.InterfaceDescription,
	v260oom.InterfaceDescription,
	v260oomprekill.InterfaceDescription,
	v260pcrextend.InterfaceDescription,
	v260pcrlock.InterfaceDescription,
	v260repart.InterfaceDescription,
	v260resolve.InterfaceDescription,
	v260resolvehook.InterfaceDescription,
	v260resolvemonitor.InterfaceDescription,
	v260service.InterfaceDescription,
	v260sysext.InterfaceDescription,
	v260systemd.InterfaceDescription,
	v260udev.InterfaceDescription,
	v260unit.InterfaceDescription,
	v260userdatabase.InterfaceDescription,
	v262askpassword.InterfaceDescription,
	v262bootcontrol.InterfaceDescription,
	v262credentials.InterfaceDescription,
	v262cryptenroll.InterfaceDescription,
	v262factoryreset.InterfaceDescription,
	v262hostname.InterfaceDescription,
	v262importd.InterfaceDescription,
	v262instancemetadata.InterfaceDescription,
	v262job.InterfaceDescription,
	v262journal.InterfaceDescription,
	v262journalaccess.InterfaceDescription,
	v262login.InterfaceDescription,
	v262machine.InterfaceDescription,
	v262machineimage.InterfaceDescription,
	v262machineinstance.InterfaceDescription,
	v262managedoom.InterfaceDescription,
	v262manager.InterfaceDescription,
	v262metrics.InterfaceDescription,
	v262mountfilesystem.InterfaceDescription,
	v262muteconsole.InterfaceDescription,
	v262namespaceresource.InterfaceDescription,
	v262network.InterfaceDescription,
	v262networklink.InterfaceDescription,
	v262oom.InterfaceDescription,
	v262oomprekill.InterfaceDescription,
	v262pcrextend.InterfaceDescription,
	v262pcrlock.InterfaceDescription,
	v262repart.InterfaceDescription,
	v262report.InterfaceDescription,
	v262reportsigner.InterfaceDescription,
	v262reportuploader.InterfaceDescription,
	v262resolve.InterfaceDescription,
	v262resolvehook.InterfaceDescription,
	v262resolvemonitor.InterfaceDescription,
	v262service.InterfaceDescription,
	v262shutdown.InterfaceDescription,
	v262storageprovider.InterfaceDescription,
	v262sysext.InterfaceDescription,
	v262sysinstall.InterfaceDescription,
	v262systemd.InterfaceDescription,
	v262sysupdate.InterfaceDescription,
	v262sysupdatenotify.InterfaceDescription,
	v262udev.InterfaceDescription,
	v262unit.InterfaceDescription,
	v262userdatabase.InterfaceDescription,
	v262virtualmachineinstance.InterfaceDescription,
}
