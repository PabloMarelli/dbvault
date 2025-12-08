# dbvault

A CLI tool for securely retrieving database credentials from HashiCorp Vault using Bitwarden for authentication.

## Overview

`dbvault` simplifies the workflow of accessing rotating database credentials stored in Vault by:
- Using Bitwarden CLI to securely store and retrieve Vault authentication credentials
- Fetching dynamic database credentials from HashiCorp Vault
- Automatically copying credentials to clipboard
- Optionally updating Neovim database connection configurations

## Prerequisites

### Required Tools
- **Bitwarden CLI** - [Install Guide](https://bitwarden.com/help/cli/)
  ```bash
  # macOS
  brew install bitwarden-cli
  
  # Verify installation
  bw --version
  ```

- **HashiCorp Vault Access** - You must have:
  - Valid Vault user account with userpass auth enabled
  - Access to the database credential paths (e.g., `database/static-creds/ops-*`)

### Bitwarden Setup

Create Bitwarden items for each environment (prod, sqa, dev) with the following structure:

**Item Name**: `vault-{environment}` (e.g., `vault-prod`)

**Login Fields**:
- Username: Your Vault username
- Password: Your Vault password

**Custom Fields**:
- `URL` (text): Vault server URL (e.g., `https://vault.example.com`)
- `DB-URL` (text): Database connection template (e.g., `postgresql://{{username}}:{{password}}@host:5432/{{database}}`)

## Installation

### Option 1: Build from source
```bash
git clone <repository-url>
cd dbvault
go build -o dbvault
sudo mv dbvault /usr/local/bin/
```

### Option 2: Go install
```bash
go install <repository-url>@latest
```

## Usage

### Basic Usage - Copy Password
```bash
dbvault
```
This will:
1. Prompt you to select an environment (prod/sqa/dev)
2. Prompt you to select a database
3. Unlock Bitwarden (if locked)
4. Retrieve credentials from Vault
5. Copy the database password to clipboard

### Copy Full Connection URL
```bash
dbvault --url
# or
dbvault -u
```
Copies the complete database connection string (including credentials) to clipboard.

### Update Neovim Database Config
```bash
dbvault --setNvimDB
# or
dbvault -n
```
Updates your Neovim database configuration file at `~/.config/dbqueries/connections.json`.

### Combine Flags
```bash
dbvault -u -n
```
Copies URL to clipboard AND updates Neovim config.

## Available Databases

- accounts
- billing
- calls
- pingpost-1, pingpost-2, pingpost-3
- reports
- webleads

## How It Works

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Bitwarden   │────▶│    Vault     │────▶│   Database   │
│  (Auth)      │     │ (Creds)      │     │  (Access)    │
└──────────────┘     └──────────────┘     └──────────────┘
      │                     │                     │
      │                     │                     │
   1. Unlock            2. Fetch             3. Connect
   w/ master           dynamic              with temp
   password            creds                credentials
```

1. **Bitwarden** stores your Vault authentication credentials
2. **dbvault** uses those credentials to authenticate to Vault
3. **Vault** generates dynamic database credentials with limited TTL
4. Credentials are copied to clipboard for use

## Security Features

- ✅ No credentials stored in code or config files
- ✅ Bitwarden session tokens cached with 0600 permissions
- ✅ Passwords copied to clipboard (not printed to terminal)
- ✅ Dynamic credentials with automatic rotation via Vault
- ✅ Session tokens stored in `~/.cache/.bw_session/` (not in shell history)

## Troubleshooting

### "Vault is locked"
```bash
bw login
bw unlock
```

### "Not authenticated to Vault"
Check your Bitwarden item has correct Vault credentials:
```bash
bw get item vault-prod
```

### "Database connection failed"
Verify you have access to the Vault path:
```bash
vault read database/static-creds/ops-{database-name}
```

### Session token issues
Clear cached session:
```bash
rm ~/.cache/.bw_session/.bw_session
```

## Development

### Project Structure
```
dbvault/
├── main.go                    # CLI entry point & database selection
├── cmd/
│   ├── bitwarden/
│   │   ├── bwmanager.go      # Bitwarden item retrieval
│   │   └── bwsession.go      # Session management & auth
│   ├── vault/
│   │   └── vault.go          # Vault API interactions
│   └── utils/
│       └── utils.go          # Utility functions
├── go.mod
└── go.sum
```

### Running Tests
```bash
go test ./...
```

### Building
```bash
go build -o dbvault
```

## Contributing

1. Create a feature branch
2. Make your changes
3. Test thoroughly with all environments
4. Submit a pull request

## License

See [LICENSE](LICENSE) file for details.

## Support

For issues or questions, contact the DevOps team or open an issue in the repository.
