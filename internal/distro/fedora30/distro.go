package fedora30

import (
	"errors"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Fedora30 struct {
	outputs map[string]output
}

type output struct {
	Name             string
	MimeType         string
	Packages         []string
	ExcludedPackages []string
	EnabledServices  []string
	DisabledServices []string
	KernelOptions    string
	IncludeFSTab     bool
	Assembler        *pipeline.Assembler
}

func New() *Fedora30 {
	r := Fedora30{
		outputs: map[string]output{},
	}

	r.outputs["ami"] = output{
		Name:     "image.raw.xz",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"langpacks-en",
			"libxcrypt-compat",
			"xfsprogs",
			"cloud-init",
			"checkpolicy",
			"net-tools",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		EnabledServices: []string{
			"cloud-init.service",
		},
		KernelOptions: "ro no_timer_check console=ttyS0,115200n8 console=tty1 biosdevname=0 net.ifnames=0 console=ttyS0,115200",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("raw.xz", "image.raw.xz"),
	}

	r.outputs["ext4-filesystem"] = output{
		Name:     "filesystem.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  false,
		Assembler:     r.rawFSAssembler("filesystem.img"),
	}

	r.outputs["partitioned-disk"] = output{
		Name:     "disk.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"grub2-pc",
			"kernel",
			"langpacks-en",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("raw", "disk.img"),
	}

	r.outputs["qcow2"] = output{
		Name:     "image.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"kernel-core",
			"@Fedora Cloud Server",
			"chrony",
			"polkit",
			"systemd-udev",
			"selinux-policy-targeted",
			"grub2-pc",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"gobject-introspection",
			"plymouth",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("qcow2", "image.qcow2"),
	}

	r.outputs["openstack"] = output{
		Name:     "image.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"spice-vdagent",
			"qemu-guest-agent",
			"xen-libs",
			"langpacks-en",
			"cloud-init",
			"libdrm",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("qcow2", "image.qcow2"),
	}

	r.outputs["tar"] = output{
		Name:     "root.tar.xz",
		MimeType: "application/x-tar",
		Packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  false,
		Assembler:     r.tarAssembler("root.tar.xz", "xz"),
	}

	r.outputs["vhd"] = output{
		Name:     "image.vhd",
		MimeType: "application/x-vhd",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"langpacks-en",
			"net-tools",
			"ntfsprogs",
			"WALinuxAgent",
			"libxcrypt-compat",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("vpc", "image.vhd"),
	}

	r.outputs["vmdk"] = output{
		Name:     "disk.vmdk",
		MimeType: "application/x-vmdk",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"grub2-pc",
			"kernel",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		IncludeFSTab:  true,
		Assembler:     r.qemuAssembler("vmdk", "disk.vmdk"),
	}

	return &r
}

func (r *Fedora30) Repositories() []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:       "fedora",
			Name:     "Fedora 30",
			Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever&arch=$basearch",
			Checksum: "sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97",
			GPGKey: `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFturGcBEACv0xBo91V2n0uEC2vh69ywCiSyvUgN/AQH8EZpCVtM7NyjKgKm
bbY4G3R0M3ir1xXmvUDvK0493/qOiFrjkplvzXFTGpPTi0ypqGgxc5d0ohRA1M75
L+0AIlXoOgHQ358/c4uO8X0JAA1NYxCkAW1KSJgFJ3RjukrfqSHWthS1d4o8fhHy
KJKEnirE5hHqB50dafXrBfgZdaOs3C6ppRIePFe2o4vUEapMTCHFw0woQR8Ah4/R
n7Z9G9Ln+0Cinmy0nbIDiZJ+pgLAXCOWBfDUzcOjDGKvcpoZharA07c0q1/5ojzO
4F0Fh4g/BUmtrASwHfcIbjHyCSr1j/3Iz883iy07gJY5Yhiuaqmp0o0f9fgHkG53
2xCU1owmACqaIBNQMukvXRDtB2GJMuKa/asTZDP6R5re+iXs7+s9ohcRRAKGyAyc
YKIQKcaA+6M8T7/G+TPHZX6HJWqJJiYB+EC2ERblpvq9TPlLguEWcmvjbVc31nyq
SDoO3ncFWKFmVsbQPTbP+pKUmlLfJwtb5XqxNR5GEXSwVv4I7IqBmJz1MmRafnBZ
g0FJUtH668GnldO20XbnSVBr820F5SISMXVwCXDXEvGwwiB8Lt8PvqzXnGIFDAu3
DlQI5sxSqpPVWSyw08ppKT2Tpmy8adiBotLfaCFl2VTHwOae48X2dMPBvQARAQAB
tDFGZWRvcmEgKDMwKSA8ZmVkb3JhLTMwLXByaW1hcnlAZmVkb3JhcHJvamVjdC5v
cmc+iQI4BBMBAgAiBQJbbqxnAhsPBgsJCAcDAgYVCAIJCgsEFgIDAQIeAQIXgAAK
CRDvPBEfz8ZZudTnD/9170LL3nyTVUCFmBjT9wZ4gYnpwtKVPa/pKnxbbS+Bmmac
g9TrT9pZbqOHrNJLiZ3Zx1Hp+8uxr3Lo6kbYwImLhkOEDrf4aP17HfQ6VYFbQZI8
f79OFxWJ7si9+3gfzeh9UYFEqOQfzIjLWFyfnas0OnV/P+RMQ1Zr+vPRqO7AR2va
N9wg+Xl7157dhXPCGYnGMNSoxCbpRs0JNlzvJMuAea5nTTznRaJZtK/xKsqLn51D
K07k9MHVFXakOH8QtMCUglbwfTfIpO5YRq5imxlWbqsYWVQy1WGJFyW6hWC0+RcJ
Ox5zGtOfi4/dN+xJ+ibnbyvy/il7Qm+vyFhCYqIPyS5m2UVJUuao3eApE38k78/o
8aQOTnFQZ+U1Sw+6woFTxjqRQBXlQm2+7Bt3bqGATg4sXXWPbmwdL87Ic+mxn/ml
SMfQux/5k6iAu1kQhwkO2YJn9eII6HIPkW+2m5N1JsUyJQe4cbtZE5Yh3TRA0dm7
+zoBRfCXkOW4krchbgww/ptVmzMMP7GINJdROrJnsGl5FVeid9qHzV7aZycWSma7
CxBYB1J8HCbty5NjtD6XMYRrMLxXugvX6Q4NPPH+2NKjzX4SIDejS6JjgrP3KA3O
pMuo7ZHMfveBngv8yP+ZD/1sS6l+dfExvdaJdOdgFCnp4p3gPbw5+Lv70HrMjA==
=BfZ/
-----END PGP PUBLIC KEY BLOCK-----
`,
		},
	}
}

func (r *Fedora30) ListOutputFormats() []string {
	formats := make([]string, 0, len(r.outputs))
	for name := range r.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (r *Fedora30) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.outputs[outputFormat]; exists {
		return output.Name, output.MimeType, nil
	}
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (r *Fedora30) Pipeline(b *blueprint.Blueprint, outputFormat string) (*pipeline.Pipeline, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, errors.New("invalid output format: " + outputFormat)
	}

	p := &pipeline.Pipeline{}
	p.SetBuild(r.buildPipeline(), "org.osbuild.fedora30")

	packages := append(output.Packages, b.GetPackages()...)
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(packages, output.ExcludedPackages)))
	p.AddStage(pipeline.NewFixBLSStage())

	// TODO support setting all languages and install corresponding langpack-* package
	language, keyboard := b.GetPrimaryLocale()

	if language != nil {
		p.AddStage(pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{*language}))
	} else {
		p.AddStage(pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{"en_US"}))
	}

	if keyboard != nil {
		p.AddStage(pipeline.NewKeymapStage(&pipeline.KeymapStageOptions{*keyboard}))
	}

	if hostname := b.GetHostname(); hostname != nil {
		p.AddStage(pipeline.NewHostnameStage(&pipeline.HostnameStageOptions{*hostname}))
	}

	timezone, ntpServers := b.GetTimezoneSettings()

	// TODO install chrony when this is set?
	if timezone != nil {
		p.AddStage(pipeline.NewTimezoneStage(&pipeline.TimezoneStageOptions{*timezone}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(pipeline.NewChronyStage(&pipeline.ChronyStageOptions{ntpServers}))
	}

	if users := b.GetUsers(); len(users) > 0 {
		options, err := r.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(pipeline.NewUsersStage(options))
	}

	if groups := b.GetGroups(); len(groups) > 0 {
		p.AddStage(pipeline.NewGroupsStage(r.groupStageOptions(groups)))
	}

	if output.IncludeFSTab {
		p.AddStage(pipeline.NewFSTabStage(r.fsTabStageOptions()))
	}
	p.AddStage(pipeline.NewGRUB2Stage(r.grub2StageOptions(output.KernelOptions, b.GetKernel())))

	if services := b.GetServices(); services != nil || output.EnabledServices != nil {
		p.AddStage(pipeline.NewSystemdStage(r.systemdStageOptions(output.EnabledServices, output.DisabledServices, services)))
	}

	if firewall := b.GetFirewall(); firewall != nil {
		p.AddStage(pipeline.NewFirewallStage(r.firewallStageOptions(firewall)))
	}

	p.AddStage(pipeline.NewSELinuxStage(r.selinuxStageOptions()))
	p.Assembler = output.Assembler

	return p, nil
}

func (r *Fedora30) Runner() string {
	return "org.osbuild.fedora30"
}

func (r *Fedora30) buildPipeline() *pipeline.Pipeline {
	packages := []string{
		"dnf",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"grub2-pc",
		"tar",
	}
	p := &pipeline.Pipeline{}
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(packages, nil)))
	return p
}

func (r *Fedora30) dnfStageOptions(packages, excludedPackages []string) *pipeline.DNFStageOptions {
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	for _, repo := range r.Repositories() {
		options.AddRepository(&pipeline.DNFRepository{
			BaseURL:    repo.BaseURL,
			MetaLink:   repo.Metalink,
			MirrorList: repo.MirrorList,
			Checksum:   repo.Checksum,
			GPGKey:     repo.GPGKey,
		})
	}

	for _, pkg := range packages {
		options.AddPackage(pkg)
	}

	for _, pkg := range excludedPackages {
		options.ExcludePackage(pkg)
	}

	return options
}

func (r *Fedora30) userStageOptions(users []blueprint.UserCustomization) (*pipeline.UsersStageOptions, error) {
	options := pipeline.UsersStageOptions{
		Users: make(map[string]pipeline.UsersStageOptionsUser),
	}

	for _, c := range users {
		if c.Password != nil && !crypt.PasswordIsCrypted(*c.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*c.Password)
			if err != nil {
				return nil, err
			}

			c.Password = &cryptedPassword
		}

		user := pipeline.UsersStageOptionsUser{
			Groups:      c.Groups,
			Description: c.Description,
			Home:        c.Home,
			Shell:       c.Shell,
			Password:    c.Password,
			Key:         c.Key,
		}

		if c.UID != nil {
			uid := strconv.Itoa(*c.UID)
			user.UID = &uid
		}

		if c.GID != nil {
			gid := strconv.Itoa(*c.GID)
			user.GID = &gid
		}

		options.Users[c.Name] = user
	}

	return &options, nil
}

func (r *Fedora30) groupStageOptions(groups []blueprint.GroupCustomization) *pipeline.GroupsStageOptions {
	options := pipeline.GroupsStageOptions{
		Groups: map[string]pipeline.GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		groupData := pipeline.GroupsStageOptionsGroup{
			Name: group.Name,
		}
		if group.GID != nil {
			gid := strconv.Itoa(*group.GID)
			groupData.GID = &gid
		}

		options.Groups[group.Name] = groupData
	}

	return &options
}

func (r *Fedora30) firewallStageOptions(firewall *blueprint.FirewallCustomization) *pipeline.FirewallStageOptions {
	options := pipeline.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *Fedora30) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *pipeline.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		enabledServices = append(disabledServices, s.Disabled...)
	}
	return &pipeline.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (r *Fedora30) fsTabStageOptions() *pipeline.FSTabStageOptions {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	options := pipeline.FSTabStageOptions{}
	options.AddFilesystem(id, "ext4", "/", "defaults", 1, 1)
	return &options
}

func (r *Fedora30) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization) *pipeline.GRUB2StageOptions {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}

	if kernel != nil {
		kernelOptions += " " + kernel.Append
	}

	return &pipeline.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
	}
}

func (r *Fedora30) selinuxStageOptions() *pipeline.SELinuxStageOptions {
	return &pipeline.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *Fedora30) qemuAssembler(format string, filename string) *pipeline.Assembler {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	return pipeline.NewQEMUAssembler(
		&pipeline.QEMUAssemblerOptions{
			Format:             format,
			Filename:           filename,
			PTUUID:             "0x14fc63d2",
			RootFilesystemUUDI: id,
			Size:               3222274048,
		})
}

func (r *Fedora30) tarAssembler(filename, compression string) *pipeline.Assembler {
	return pipeline.NewTarAssembler(
		&pipeline.TarAssemblerOptions{
			Filename: filename,
		})
}

func (r *Fedora30) rawFSAssembler(filename string) *pipeline.Assembler {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	return pipeline.NewRawFSAssembler(
		&pipeline.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               3222274048,
		})
}
