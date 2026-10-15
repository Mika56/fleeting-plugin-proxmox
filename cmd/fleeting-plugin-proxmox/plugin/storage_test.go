package plugin

import (
	"testing"

	"github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/require"
)

const (
	storageTestStorage = "lvm"
	storageTestDisk    = "sata0"
)

func TestParseProxmoxSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		size    string
		want    uint64
		wantErr bool
	}{
		{size: "1073741824", want: 1 << 30},
		{size: "512K", want: 512 << 10},
		{size: "788M", want: 788 << 20},
		{size: deployTestAutoresizeSize, want: 10 << 30},
		{size: "1.5G", want: 3 << 29},
		{size: "2T", want: 2 << 40},
		{size: "+10G", wantErr: true},
		{size: "10GB", wantErr: true},
		{size: "", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.size, func(t *testing.T) {
			t.Parallel()

			got, err := parseProxmoxSize(testCase.size)
			if testCase.wantErr {
				require.ErrorIs(t, err, ErrInvalidSize)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestDiskSpecSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		spec          string
		want          uint64
		wantAllocates bool
	}{
		{name: "disk", spec: "local-lvm:vm-100-disk-0,discard=on,size=32G", want: 32 << 30, wantAllocates: true},
		{name: "size first", spec: "local-lvm:vm-100-disk-0,size=4M,efitype=4m", want: 4 << 20, wantAllocates: true},
		{name: "iso", spec: "local:iso/alpine.iso,media=cdrom,size=60M"},
		{name: "cloud-init drive", spec: "local-lvm:vm-100-cloudinit,media=cdrom"},
		{name: "empty drive", spec: "none,media=cdrom"},
		{name: "no size", spec: "local-lvm:vm-100-disk-0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, allocates, err := diskSpecSize(testCase.spec)
			require.NoError(t, err)
			require.Equal(t, testCase.wantAllocates, allocates)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestInstanceFootprint(t *testing.T) {
	t.Parallel()

	template := &proxmox.VirtualMachine{VirtualMachineConfig: &proxmox.VirtualMachineConfig{
		SATAs:    map[string]string{storageTestDisk: "lvm:base-1001-disk-0,size=1G"},
		SCSIs:    map[string]string{"scsi1": "lvm:base-1001-disk-1,size=2G"},
		IDEs:     map[string]string{"ide2": "none,media=cdrom"},
		EFIDisk0: "lvm:base-1001-disk-2,size=4M",
	}}

	testCases := []struct {
		name           string
		autoresizeDisk string
		autoresizeSize string
		want           uint64
		wantErr        error
	}{
		{name: "no autoresize", want: 3<<30 + 4<<20},
		{name: "absolute", autoresizeDisk: storageTestDisk, autoresizeSize: deployTestAutoresizeSize, want: 12<<30 + 4<<20},
		{name: "absolute below current", autoresizeDisk: "scsi1", autoresizeSize: "1G", want: 3<<30 + 4<<20},
		{name: "increment", autoresizeDisk: storageTestDisk, autoresizeSize: "+5G", want: 8<<30 + 4<<20},
		{name: "missing disk", autoresizeDisk: "virtio0", autoresizeSize: deployTestAutoresizeSize, wantErr: ErrDiskNotFound},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ig := newWaitTestGroup()
			ig.InstanceAutoresizeDisk = testCase.autoresizeDisk
			ig.InstanceAutoresizeSize = testCase.autoresizeSize

			got, err := ig.instanceFootprint(template)
			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestValidateStorageReserve(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		settings Settings
		wantErr  bool
	}{
		{name: "unset", settings: Settings{}},
		{name: "reserve with storage", settings: Settings{StorageReserve: "5G", Storage: storageTestStorage}},
		{name: "zero reserve with storage", settings: Settings{StorageReserve: "0", Storage: storageTestStorage}},
		{name: "reserve without storage", settings: Settings{StorageReserve: "5G"}, wantErr: true},
		{name: "reserve increment", settings: Settings{StorageReserve: "+5G", Storage: storageTestStorage}, wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			settings := testCase.settings

			err := settings.validateStorageReserve()
			if testCase.wantErr {
				require.ErrorIs(t, err, ErrSettingInvalidParameter)

				return
			}

			require.NoError(t, err)
		})
	}
}
