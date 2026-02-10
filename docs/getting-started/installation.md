# Installation

Choose your preferred installation method.

## Homebrew (macOS/Linux)

```bash
brew install vanillacake369/tap/agent-collab
```

## APT (Debian/Ubuntu)

```bash
curl -fsSL https://vanillacake369.github.io/agent-collab/gpg.key | \
  sudo gpg --dearmor -o /usr/share/keyrings/agent-collab.gpg

echo "deb [signed-by=/usr/share/keyrings/agent-collab.gpg] \
  https://vanillacake369.github.io/agent-collab stable main" | \
  sudo tee /etc/apt/sources.list.d/agent-collab.list

sudo apt update && sudo apt install agent-collab
```

## Go Install

```bash
go install github.com/vanillacake369/agent-collab/src@latest
```

## Docker

```bash
docker pull ghcr.io/vanillacake369/agent-collab:latest
```

## Other Methods

### RPM (Fedora/RHEL)

```bash
curl -fsSL https://github.com/vanillacake369/agent-collab/releases/latest/download/agent-collab_linux_amd64.rpm -o agent-collab.rpm
sudo rpm -i agent-collab.rpm
```

### Binary Download

Download from [Releases](https://github.com/vanillacake369/agent-collab/releases):

| Platform | File |
|----------|------|
| macOS Apple Silicon | `agent-collab_vX.Y.Z_darwin_arm64.tar.gz` |
| macOS Intel | `agent-collab_vX.Y.Z_darwin_amd64.tar.gz` |
| Linux x86_64 | `agent-collab_vX.Y.Z_linux_amd64.tar.gz` |
| Linux ARM64 | `agent-collab_vX.Y.Z_linux_arm64.tar.gz` |
| Windows | `agent-collab_vX.Y.Z_windows_amd64.zip` |

### Build from Source

```bash
git clone https://github.com/vanillacake369/agent-collab.git
cd agent-collab
go build -o agent-collab ./src
```

## Verify Installation

```bash
agent-collab --version
```

## Next Steps

Continue to [Quick Start](quick-start.md) to set up your first cluster.
