# Network and USB transfer configuration

## Network roles

```text
Internet / PPPoE
       |
MikroTik LHG5: 192.168.50.2 (gateway)
       | LAN cable
NW-651D LAN port: 192.168.50.1 (Wi-Fi, LAN switch, USB services)
       |
Home computers and phones
```

Use a LAN-to-LAN connection for this arrangement. Using the NW-651D WAN port would require a different routing design and can create a second NAT layer. The gateway handles Internet/PPPoE; the NW-651D is also a service host, not just a passive switch.

Keep both management addresses outside the DHCP pool. In the original configuration the storage host ran DHCP, rather than the gateway. Run only one intended DHCP server on a shared broadcast domain.

| Storage-host setting | Example |
| --- | --- |
| LAN address / mask | `192.168.50.1/24` |
| Its default route | `0.0.0.0/0` via `192.168.50.2`, metric 1, `br0` |
| DHCP pool | `192.168.50.100`–`192.168.50.244` |
| DHCP default gateway | `192.168.50.2` |
| DHCP DNS | `1.1.1.1`, `8.8.8.8` |
| Gateway LAN address | `192.168.50.2/24` |
| Gateway Internet interface | `pppoe-out1` |
| Gateway LAN interface | `ether1` |

Save the static route and DHCP settings through the firmware's supported configuration mechanism. Renew client leases to receive changed DHCP options. A `route add` command alone normally does not survive reboot. Retain existing leases/ARP bindings unless deliberately changing them.

On the storage host, inspect `route -n`, the active dnsmasq configuration, and `ping -c 1 -W 2 192.168.50.2`. Check a known HTTPS website from both the host and a client. A panel may still report WAN disconnected because the WAN port is unused; do not change an otherwise working topology solely to change that badge.

## SSH and SFTP

Configure a private SSH alias on your computer, for example in `~/.ssh/config`:

```sshconfig
Host router
    HostName 192.168.50.1
    Port 22
    User admin
    IdentityFile ~/.ssh/your_private_key
```

Never commit the private key or a real SSH configuration to the repository. Verify the router's host key before first use. The observed firmware provided `/usr/libexec/sftp-server`.

```sh
ssh router
sftp router
```

A separate gateway forward can map WAN TCP 222 to the storage host's TCP 22. Scope that rule to `pppoe-out1`, use key authentication, and inspect the existing SSH configuration before exposing it. This mapping does not configure SSH authentication by itself. An external SFTP client then uses the public hostname, port 222, and the authorized SSH account. Do not assume port 22 on the public address reaches the storage host.

SFTP encrypts credentials and transferred data. Plain FTP does not. For public or untrusted-network transfers, use SFTP rather than exposing the plain FTP service.

## Inspect USB storage

```sh
mount
cat /proc/mounts
df -h /media/DISK_ID
ls -ld /media/DISK_ID /media/DISK_ID/public-storage
```

The original disk was exFAT, mounted read/write. The mount used `fmask=0000,dmask=0000`: Unix permission bits and `chmod 600` are not reliable isolation on this filesystem. Account restrictions and service path boundaries matter. An authenticated storage account that can read the whole disk may also read the service's private keys outside the public directory. Restrict account scope or use a filesystem with enforced permissions when stronger local isolation is needed.

Use a unique disposable probe, not an existing filename:

```sh
probe_dir=/media/DISK_ID/usb-probe-CHOOSE_A_UNIQUE_SUFFIX
mkdir "$probe_dir" || exit 1
printf 'USB read/write probe\n' > "$probe_dir/probe.txt"
cat "$probe_dir/probe.txt"
rm "$probe_dir/probe.txt"
rmdir "$probe_dir"
```

Only remove files created for the test. If the disk is read-only, inspect mount flags and firmware logs before retrying. Back up valuable data and perform filesystem repair while unmounted, preferably on a computer with appropriate exFAT tools.

## FTP settings and a real transfer test

The observed vsftpd settings were:

```ini
local_enable=YES
anonymous_enable=NO
write_enable=YES
pasv_min_port=50000
pasv_max_port=50100
```

These flags permit the feature; they do not establish every user's effective directory permission. Firmware-generated configuration can overwrite direct edits, so use the panel's supported FTP and account settings. Anonymous login should fail. Do not turn it on to work around account problems.

For a LAN test, connect a client such as FileZilla to `192.168.50.1`, port 21, with an authorized FTP account. Use passive mode. Upload a unique small text file into a permitted test directory, list the directory, download it under a different local name, compare the bytes or checksum, then remove only the probe. A successful login alone is not a read/write test.

On the LAN, passive data ports must pass the host firewall. Publishing FTP would also require control/data-port forwarding and correct advertised passive addresses; this was **not configured** in the original work. Prefer SFTP for remote transfer.

To verify SFTP, use its interactive `put`, `ls`, `get`, and `rm` commands on a uniquely named probe, then compare the downloaded file locally. Resolve server filesystem/account permissions if writes fail; do not make the entire disk publicly writable.

Keep a test record stating which account, protocol, read, write, and cleanup operations succeeded. Omit passwords and private keys from that record.
