# Padavan deployment

The supervisor loads the private `/etc/storage/routershelf.env` file. Use the unchanged script and configure `.env`, not a per-network copy of shell source.

Copy `.env.example` to `.env` in the repository root. Run `--check-config` with `ROUTERSHELF_ENV_FILE` pointing to that file before installation. See [environment configuration](../../docs/configuration.md), [installation](../../docs/getting-started.md), and [operations](../../docs/operations.md).

The script targets the observed firmware hooks and firewall commands. No project edit automatically changes a running router.

## Optional independent SMB service

`smb-supervisor.example.sh` supervises only `routershelf-smb`, using `RS_SMB_BINARY` and `RS_SMB_CONFIG`. It does not load the web env, configure NTP/firewall, or touch public-share paths. See [SMB installation, lifecycle and rollback](../../docs/smb.md). Validate/approve target deployment separately; existing web hooks remain compatible.
